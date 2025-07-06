package drainer

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// Drainer handles cordoning and draining operations on nodes
type Drainer struct {
	client   kubernetes.Interface
	recorder record.EventRecorder
	config   *DrainerConfig
}

// DrainerConfig holds configuration for the drainer
type DrainerConfig struct {
	// GracePeriod is the grace period for pod termination
	GracePeriod time.Duration
	// Timeout is the maximum time to wait for drain to complete
	Timeout time.Duration
	// Force forces the drain even if there are pods that cannot be evicted
	Force bool
	// IgnoreDaemonSets ignores DaemonSet-managed pods
	IgnoreDaemonSets bool
	// DeleteEmptyDirData allows deletion of pods with emptyDir volumes
	DeleteEmptyDirData bool
	// PodSelector filters which pods to evict
	PodSelector labels.Selector
}

// NewDrainer creates a new drainer instance
func NewDrainer(client kubernetes.Interface, recorder record.EventRecorder, config *DrainerConfig) *Drainer {
	return &Drainer{
		client:   client,
		recorder: recorder,
		config:   config,
	}
}

// Cordon marks a node as unschedulable
func (d *Drainer) Cordon(ctx context.Context, node *corev1.Node) error {
	log := klog.FromContext(ctx)
	log.Info("Cordoning node", "node", node.Name)

	// Check if node is already cordoned
	if node.Spec.Unschedulable {
		log.Info("Node is already cordoned", "node", node.Name)
		return nil
	}

	// Create a patch to mark the node as unschedulable
	patch := []byte(`{"spec":{"unschedulable":true}}`)

	_, err := d.client.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Error(err, "Failed to cordon node", "node", node.Name)
		d.recorder.Eventf(node, corev1.EventTypeWarning, "CordonFailed",
			"Failed to cordon node %s: %v", node.Name, err)
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	log.Info("Successfully cordoned node", "node", node.Name)
	d.recorder.Eventf(node, corev1.EventTypeNormal, "Cordoned",
		"Node %s has been cordoned", node.Name)

	return nil
}

// Drain evicts all pods from a node
func (d *Drainer) Drain(ctx context.Context, node *corev1.Node) error {
	log := klog.FromContext(ctx)
	log.Info("Starting drain operation", "node", node.Name)

	// Get all pods on the node
	pods, err := d.getPodsOnNode(ctx, node.Name)
	if err != nil {
		return fmt.Errorf("failed to get pods on node: %w", err)
	}

	if len(pods) == 0 {
		log.Info("No pods to drain on node", "node", node.Name)
		return nil
	}

	log.Info("Found pods to drain", "node", node.Name, "podCount", len(pods))

	// Evict pods
	evictedPods := 0
	failedPods := 0

	for _, pod := range pods {
		if err := d.evictPod(ctx, &pod); err != nil {
			log.Error(err, "Failed to evict pod", "node", node.Name, "pod", pod.Name, "namespace", pod.Namespace)
			failedPods++

			if !d.config.Force {
				return fmt.Errorf("failed to evict pod %s/%s: %w", pod.Namespace, pod.Name, err)
			}
		} else {
			evictedPods++
		}
	}

	log.Info("Drain operation completed", "node", node.Name, "evictedPods", evictedPods, "failedPods", failedPods)

	if failedPods > 0 {
		d.recorder.Eventf(node, corev1.EventTypeWarning, "DrainIncomplete",
			"Drain completed with %d failed pod evictions on node %s", failedPods, node.Name)
	} else {
		d.recorder.Eventf(node, corev1.EventTypeNormal, "DrainCompleted",
			"Successfully drained %d pods from node %s", evictedPods, node.Name)
	}

	return nil
}

// getPodsOnNode gets all pods running on the specified node
func (d *Drainer) getPodsOnNode(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName)

	pods, err := d.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
		LabelSelector: d.config.PodSelector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods on node: %w", err)
	}

	// Filter out pods that should be ignored
	var filteredPods []corev1.Pod
	for _, pod := range pods.Items {
		if d.shouldEvictPod(&pod) {
			filteredPods = append(filteredPods, pod)
		}
	}

	return filteredPods, nil
}

// shouldEvictPod determines if a pod should be evicted
func (d *Drainer) shouldEvictPod(pod *corev1.Pod) bool {
	// Skip pods that are already terminating
	if pod.DeletionTimestamp != nil {
		return false
	}

	// Skip mirror pods
	if pod.Annotations["kubernetes.io/config.mirror"] != "" {
		return false
	}

	// Skip DaemonSet pods if configured to ignore them
	if d.config.IgnoreDaemonSets {
		if pod.OwnerReferences != nil {
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "DaemonSet" {
					return false
				}
			}
		}
	}

	// Skip pods with local storage unless force is enabled
	if d.hasLocalStorage(pod) && !d.config.Force {
		return false
	}

	return true
}

// hasLocalStorage checks if a pod has local storage
func (d *Drainer) hasLocalStorage(pod *corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.EmptyDir != nil || volume.HostPath != nil {
			return true
		}
	}
	return false
}

// evictPod evicts a single pod
func (d *Drainer) evictPod(ctx context.Context, pod *corev1.Pod) error {
	log := klog.FromContext(ctx)
	log.Info("Evicting pod", "pod", pod.Name, "namespace", pod.Namespace, "node", pod.Spec.NodeName)

	// Create eviction object
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{
			GracePeriodSeconds: &[]int64{int64(d.config.GracePeriod.Seconds())}[0],
		},
	}

	// Perform eviction
	err := d.client.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod was already deleted
			log.Info("Pod was already deleted", "pod", pod.Name, "namespace", pod.Namespace)
			return nil
		}
		return fmt.Errorf("failed to evict pod: %w", err)
	}

	log.Info("Successfully evicted pod", "pod", pod.Name, "namespace", pod.Namespace)
	return nil
}

// Uncordon marks a node as schedulable
func (d *Drainer) Uncordon(ctx context.Context, node *corev1.Node) error {
	log := klog.FromContext(ctx)
	log.Info("Uncordoning node", "node", node.Name)

	// Check if node is already uncordoned
	if !node.Spec.Unschedulable {
		log.Info("Node is already uncordoned", "node", node.Name)
		return nil
	}

	// Create a patch to mark the node as schedulable
	patch := []byte(`{"spec":{"unschedulable":false}}`)

	_, err := d.client.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Error(err, "Failed to uncordon node", "node", node.Name)
		d.recorder.Eventf(node, corev1.EventTypeWarning, "UncordonFailed",
			"Failed to uncordon node %s: %v", node.Name, err)
		return fmt.Errorf("failed to uncordon node: %w", err)
	}

	log.Info("Successfully uncordoned node", "node", node.Name)
	d.recorder.Eventf(node, corev1.EventTypeNormal, "Uncordoned",
		"Node %s has been uncordoned", node.Name)

	return nil
}
