//go:build windows
// +build windows

package cloudfiles

import (
	"testing"
)

func TestIsAvailable(t *testing.T) {
	available := IsAvailable()
	t.Logf("Cloud Files API available: %v", available)

	// On Windows 10 1709+, this should be available
	// We don't fail the test if not available, just log it
	if !available {
		t.Log("Cloud Files API not available - this is expected on older Windows versions or non-NTFS")
	}
}

func TestGetPlatformInfo(t *testing.T) {
	if !IsAvailable() {
		t.Skip("Cloud Files API not available")
	}

	info, err := GetPlatformInfo()
	if err != nil {
		t.Fatalf("GetPlatformInfo failed: %v", err)
	}

	t.Logf("Platform Info:")
	t.Logf("  Build Number: %d", info.BuildNumber)
	t.Logf("  Revision Number: %d", info.RevisionNumber)
	t.Logf("  Integration ID: %08X-%04X-%04X-%X",
		info.IntegrationId.Data1,
		info.IntegrationId.Data2,
		info.IntegrationId.Data3,
		info.IntegrationId.Data4)
}

func TestNewSyncRegistration(t *testing.T) {
	reg := NewSyncRegistration("AnemoneSync", "1.0.0")

	if reg == nil {
		t.Fatal("NewSyncRegistration returned nil")
	}

	if reg.StructSize == 0 {
		t.Error("StructSize should be non-zero")
	}

	if reg.ProviderName == nil {
		t.Error("ProviderName should not be nil")
	}

	if reg.ProviderVersion == nil {
		t.Error("ProviderVersion should not be nil")
	}

	t.Logf("SyncRegistration StructSize: %d bytes", reg.StructSize)
}

func TestNewDefaultSyncPolicies(t *testing.T) {
	policies := NewDefaultSyncPolicies()

	if policies == nil {
		t.Fatal("NewDefaultSyncPolicies returned nil")
	}

	if policies.StructSize == 0 {
		t.Error("StructSize should be non-zero")
	}

	if policies.Hydration.Primary != CF_HYDRATION_POLICY_FULL {
		t.Errorf("Expected FULL hydration policy, got %d", policies.Hydration.Primary)
	}

	t.Logf("SyncPolicies StructSize: %d bytes", policies.StructSize)
	t.Logf("  Hydration: Primary=%d, Modifier=0x%04X", policies.Hydration.Primary, policies.Hydration.Modifier)
	t.Logf("  Population: Primary=%d", policies.Population.Primary)
	t.Logf("  InSync: 0x%08X", policies.InSync)
}

func TestCallbackRegistrationEnd(t *testing.T) {
	if CF_CALLBACK_REGISTRATION_END.Type != CF_CALLBACK_TYPE_NONE {
		t.Errorf("Expected CF_CALLBACK_TYPE_NONE, got %d", CF_CALLBACK_REGISTRATION_END.Type)
	}

	if CF_CALLBACK_REGISTRATION_END.Callback != 0 {
		t.Errorf("Expected callback to be nil (0), got %v", CF_CALLBACK_REGISTRATION_END.Callback)
	}
}

func TestPlaceholderStateConstants(t *testing.T) {
	// Just verify constants are defined correctly
	states := map[string]CF_PLACEHOLDER_STATE{
		"NO_STATES":         CF_PLACEHOLDER_STATE_NO_STATES,
		"PLACEHOLDER":       CF_PLACEHOLDER_STATE_PLACEHOLDER,
		"SYNC_ROOT":         CF_PLACEHOLDER_STATE_SYNC_ROOT,
		"IN_SYNC":           CF_PLACEHOLDER_STATE_IN_SYNC,
		"PARTIAL":           CF_PLACEHOLDER_STATE_PARTIAL,
		"PARTIALLY_ON_DISK": CF_PLACEHOLDER_STATE_PARTIALLY_ON_DISK,
		"INVALID":           CF_PLACEHOLDER_STATE_INVALID,
	}

	for name, state := range states {
		t.Logf("CF_PLACEHOLDER_STATE_%s = 0x%08X", name, state)
	}
}

func TestPinStateConstants(t *testing.T) {
	states := map[string]CF_PIN_STATE{
		"UNSPECIFIED": CF_PIN_STATE_UNSPECIFIED,
		"PINNED":      CF_PIN_STATE_PINNED,
		"UNPINNED":    CF_PIN_STATE_UNPINNED,
		"EXCLUDED":    CF_PIN_STATE_EXCLUDED,
		"INHERIT":     CF_PIN_STATE_INHERIT,
	}

	for name, state := range states {
		t.Logf("CF_PIN_STATE_%s = %d", name, state)
	}
}
