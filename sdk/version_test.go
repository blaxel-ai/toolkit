package sdk

import (
	"testing"
)

func TestGetVersion(t *testing.T) {
	version := GetVersion()

	// Version should not be empty
	if version == "" {
		t.Error("GetVersion() returned empty string")
	}

	// Should return consistent value on multiple calls
	version2 := GetVersion()
	if version != version2 {
		t.Errorf("GetVersion() returned different values: %s vs %s", version, version2)
	}

	t.Logf("Detected version: %s", version)
}
