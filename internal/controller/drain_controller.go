package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/nfelsen/draino2/internal/drainer"
	"github.com/nfelsen/draino2/internal/metrics"
	"github.com/nfelsen/draino2/internal/types"
)

// DrainController reconciles Node objects to handle draining based on labels and conditions
type DrainController struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   *types.Config
	Drainer  *drainer.Drainer
	Metrics  *metrics.Metrics
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile handles the reconciliation of a Node
func (r *DrainController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)
	log.Info("Reconciling node", "node", req.Name)

	// Fetch the Node
	node := &corev1.Node{}
	err := r.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if errors.IsNotFound(err) {
			// Node was deleted, nothing to do
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get node")
		return ctrl.Result{}, err
	}

	// Check if node should be drained based on labels
	shouldDrain, reason := r.shouldDrainNode(node)
	if !shouldDrain {
		log.V(2).Info("Node should not be drained", "node", node.Name, "reason", reason)
		return ctrl.Result{}, nil
	}

	// Check if node is already being drained or has been drained
	if r.isNodeBeingDrained(node) {
		log.Info("Node is already being drained", "node", node.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if r.isNodeDrained(node) {
		log.Info("Node has already been drained", "node", node.Name)
		return ctrl.Result{}, nil
	}

	// Start draining the node
	log.Info("Starting drain operation", "node", node.Name, "reason", reason)

	// Record audit event
	r.recordDrainStart(node, reason)

	// Perform the drain operation
	err = r.performDrain(ctx, node, reason)
	if err != nil {
		log.Error(err, "Failed to drain node", "node", node.Name)
		r.recordDrainFailure(node, reason, err)
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	log.Info("Successfully drained node", "node", node.Name)
	r.recordDrainSuccess(node, reason)

	return ctrl.Result{}, nil
}

// shouldDrainNode checks if a node should be drained based on labels and conditions
func (r *DrainController) shouldDrainNode(node *corev1.Node) (bool, string) {
	// Check drain trigger labels
	for _, triggerLabel := range r.Config.LabelTriggers {
		if value, exists := node.Labels[triggerLabel.Key]; exists {
			if triggerLabel.Value == "" || value == triggerLabel.Value {
				return true, fmt.Sprintf("trigger label %s=%s", triggerLabel.Key, value)
			}
		}
	}

	// Check node conditions
	for _, condition := range node.Status.Conditions {
		if condition.Status == corev1.ConditionTrue {
			for _, drainCondition := range r.Config.NodeConditions {
				if condition.Type == drainCondition.Type {
					return true, fmt.Sprintf("condition %s is True", condition.Type)
				}
			}
		}
	}

	return false, "no drain triggers found"
}

// isNodeBeingDrained checks if a node is currently being drained
func (r *DrainController) isNodeBeingDrained(node *corev1.Node) bool {
	// Check for drain-in-progress annotation
	if _, exists := node.Annotations["draino2.kubernetes.io/drain-in-progress"]; exists {
		return true
	}
	return false
}

// isNodeDrained checks if a node has already been drained
func (r *DrainController) isNodeDrained(node *corev1.Node) bool {
	// Check for drained annotation
	if _, exists := node.Annotations["draino2.kubernetes.io/drained"]; exists {
		return true
	}
	return false
}

// performDrain performs the actual drain operation
func (r *DrainController) performDrain(ctx context.Context, node *corev1.Node, reason string) error {
	log := klog.FromContext(ctx)

	// Mark node as being drained
	if err := r.markNodeAsDraining(node); err != nil {
		return fmt.Errorf("failed to mark node as draining: %w", err)
	}

	// Perform cordon if not skipped
	if !r.Config.DrainSettings.SkipCordon {
		log.Info("Cordoning node", "node", node.Name)
		if err := r.Drainer.Cordon(ctx, node); err != nil {
			return fmt.Errorf("failed to cordon node: %w", err)
		}
	}

	// Perform drain
	log.Info("Draining node", "node", node.Name)
	if err := r.Drainer.Drain(ctx, node); err != nil {
		return fmt.Errorf("failed to drain node: %w", err)
	}

	// Mark node as drained
	if err := r.markNodeAsDrained(node, reason); err != nil {
		return fmt.Errorf("failed to mark node as drained: %w", err)
	}

	return nil
}

// markNodeAsDraining adds annotation to mark node as being drained
func (r *DrainController) markNodeAsDraining(node *corev1.Node) error {
	patch := client.MergeFrom(node.DeepCopy())

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	node.Annotations["draino2.kubernetes.io/drain-in-progress"] = "true"
	node.Annotations["draino2.kubernetes.io/drain-start-time"] = time.Now().UTC().Format(time.RFC3339)

	return r.Patch(context.Background(), node, patch)
}

// markNodeAsDrained adds annotation to mark node as drained
func (r *DrainController) markNodeAsDrained(node *corev1.Node, reason string) error {
	patch := client.MergeFrom(node.DeepCopy())

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	delete(node.Annotations, "draino2.kubernetes.io/drain-in-progress")
	node.Annotations["draino2.kubernetes.io/drained"] = "true"
	node.Annotations["draino2.kubernetes.io/drain-complete-time"] = time.Now().UTC().Format(time.RFC3339)
	node.Annotations["draino2.kubernetes.io/drain-reason"] = reason

	return r.Patch(context.Background(), node, patch)
}

// recordDrainStart records the start of a drain operation
func (r *DrainController) recordDrainStart(node *corev1.Node, reason string) {
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainStarted",
		"Drain operation started for node %s: %s", node.Name, reason)

	if r.Metrics != nil {
		r.Metrics.DrainOperationsStarted.Inc()
	}
}

// recordDrainSuccess records the successful completion of a drain operation
func (r *DrainController) recordDrainSuccess(node *corev1.Node, reason string) {
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainCompleted",
		"Drain operation completed successfully for node %s: %s", node.Name, reason)

	if r.Metrics != nil {
		r.Metrics.DrainOperationsCompleted.Inc()
	}
}

// recordDrainFailure records the failure of a drain operation
func (r *DrainController) recordDrainFailure(node *corev1.Node, reason string, err error) {
	r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed",
		"Drain operation failed for node %s: %s - %v", node.Name, reason, err)

	if r.Metrics != nil {
		r.Metrics.DrainOperationsFailed.Inc()
	}
}

// SetupWithManager sets up the controller with the given manager
func (r *DrainController) SetupWithManager(mgr ctrl.Manager) error {
	// Create predicate to filter nodes
	nodePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.shouldWatchNode(e.Object.(*corev1.Node))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode := e.ObjectOld.(*corev1.Node)
			newNode := e.ObjectNew.(*corev1.Node)

			// Check if labels changed
			if !reflect.DeepEqual(oldNode.Labels, newNode.Labels) {
				return r.shouldWatchNode(newNode)
			}

			// Check if conditions changed
			if !reflect.DeepEqual(oldNode.Status.Conditions, newNode.Status.Conditions) {
				return r.shouldWatchNode(newNode)
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false // We don't need to reconcile deleted nodes
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		WithEventFilter(nodePredicate).
		Complete(r)
}

// shouldWatchNode determines if a node should be watched based on filters
func (r *DrainController) shouldWatchNode(node *corev1.Node) bool {
	// Check skip labels
	for _, skipLabel := range r.Config.ExcludeLabels {
		if value, exists := node.Labels[skipLabel.Key]; exists {
			if skipLabel.Value == "" || value == skipLabel.Value {
				return false
			}
		}
	}

	return true
}
