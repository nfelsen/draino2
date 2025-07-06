package drainer

import (
	"testing"
	"time"
)

func TestDrainerConfig(t *testing.T) {
	config := &DrainerConfig{
		GracePeriod:        30 * time.Second,
		Timeout:            5 * time.Minute,
		Force:              false,
		IgnoreDaemonSets:   true,
		DeleteEmptyDirData: false,
		PodSelector:        nil,
	}

	if config.GracePeriod != 30*time.Second {
		t.Error("Expected GracePeriod to be 30 seconds")
	}

	if config.Timeout != 5*time.Minute {
		t.Error("Expected Timeout to be 5 minutes")
	}

	if config.Force {
		t.Error("Expected Force to be false")
	}

	if !config.IgnoreDaemonSets {
		t.Error("Expected IgnoreDaemonSets to be true")
	}

	if config.DeleteEmptyDirData {
		t.Error("Expected DeleteEmptyDirData to be false")
	}
}

func TestDrainerConfig_DefaultValues(t *testing.T) {
	config := &DrainerConfig{}

	// Test that default values are reasonable
	if config.GracePeriod < 0 {
		t.Error("GracePeriod should not be negative")
	}

	if config.Timeout < 0 {
		t.Error("Timeout should not be negative")
	}
}
