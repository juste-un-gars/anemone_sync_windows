package sync

import (
	"testing"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"go.uber.org/zap"
)

func TestNewConflictResolver(t *testing.T) {
	tests := []struct {
		name      string
		policy    string
		expectErr bool
	}{
		{"recent policy", "recent", false},
		{"local policy", "local", false},
		{"remote policy", "remote", false},
		{"ask policy", "ask", false},
		{"invalid policy", "invalid", true},
		{"empty policy", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewConflictResolver(tt.policy, zap.NewNop())

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if resolver != nil {
					t.Error("expected nil resolver on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resolver == nil {
					t.Error("expected resolver, got nil")
				}
				if resolver != nil && string(resolver.policy) != tt.policy {
					t.Errorf("expected policy %s, got %s", tt.policy, resolver.policy)
				}
			}
		})
	}
}

func TestResolveConflictsByRecent_LocalNewer(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	now := time.Now()
	localNewer := now.Add(5 * time.Minute)
	remoteOlder := now

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: localNewer,
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  90,
				MTime: remoteOlder,
			},
			NeedsResolution: true,
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}

	decision := resolved[0]
	if decision.Action != cache.ActionUpload {
		t.Errorf("expected upload action, got %s", decision.Action)
	}
	if decision.NeedsResolution {
		t.Error("resolved decision should not need resolution")
	}
}

func TestResolveConflictsByRecent_RemoteNewer(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	now := time.Now()
	localOlder := now
	remoteNewer := now.Add(5 * time.Minute)

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: localOlder,
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  110,
				MTime: remoteNewer,
			},
			NeedsResolution: true,
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}

	decision := resolved[0]
	if decision.Action != cache.ActionDownload {
		t.Errorf("expected download action, got %s", decision.Action)
	}
}

func TestResolveConflictsByRecent_SameTime_DifferentSize(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	now := time.Now()

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  200, // Larger
				MTime: now,
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100, // Smaller
				MTime: now,
			},
			NeedsResolution: true,
		},
	}

	resolved, _ := resolver.ResolveConflicts(decisions)

	decision := resolved[0]
	if decision.Action != cache.ActionUpload {
		t.Errorf("expected upload (local larger), got %s", decision.Action)
	}
}

func TestResolveConflictsByRecent_Identical(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	now := time.Now()

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: now,
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: now,
			},
			NeedsResolution: true,
		},
	}

	resolved, _ := resolver.ResolveConflicts(decisions)

	decision := resolved[0]
	if decision.Action != cache.ActionNone {
		t.Errorf("expected no action (identical), got %s", decision.Action)
	}
}

func TestResolveConflictsByLocal(t *testing.T) {
	resolver, _ := NewConflictResolver("local", zap.NewNop())

	now := time.Now()

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: now,
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  200,
				MTime: now.Add(1 * time.Hour), // Remote is newer and larger
			},
			NeedsResolution: true,
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}

	decision := resolved[0]
	if decision.Action != cache.ActionUpload {
		t.Errorf("expected upload action (local preference), got %s", decision.Action)
	}
}

func TestResolveConflictsByRemote(t *testing.T) {
	resolver, _ := NewConflictResolver("remote", zap.NewNop())

	now := time.Now()

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  200,
				MTime: now.Add(1 * time.Hour), // Local is newer and larger
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: now,
			},
			NeedsResolution: true,
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}

	decision := resolved[0]
	if decision.Action != cache.ActionDownload {
		t.Errorf("expected download action (remote preference), got %s", decision.Action)
	}
}

func TestResolveConflictsByAsk(t *testing.T) {
	resolver, _ := NewConflictResolver("ask", zap.NewNop())

	now := time.Now()

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "test.txt",
			RemotePath: "test.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  100,
				MTime: now,
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "test.txt",
				Size:  200,
				MTime: now.Add(1 * time.Hour),
			},
			NeedsResolution: true,
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	if len(resolved) != 0 {
		t.Errorf("expected 0 resolved (ask policy), got %d", len(resolved))
	}
	if len(unresolved) != 1 {
		t.Errorf("expected 1 unresolved, got %d", len(unresolved))
	}
}

func TestResolveConflictsMixed(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	now := time.Now()

	decisions := []*cache.SyncDecision{
		{
			LocalPath:  "conflict.txt",
			RemotePath: "conflict.txt",
			LocalInfo: &cache.FileInfo{
				Path:  "conflict.txt",
				Size:  100,
				MTime: now.Add(1 * time.Hour),
			},
			RemoteInfo: &cache.FileInfo{
				Path:  "conflict.txt",
				Size:  100,
				MTime: now,
			},
			NeedsResolution: true,
		},
		{
			LocalPath:  "normal.txt",
			RemotePath: "normal.txt",
			Action:     cache.ActionUpload,
			LocalInfo: &cache.FileInfo{
				Path:  "normal.txt",
				Size:  100,
				MTime: now,
			},
			NeedsResolution: false, // Not a conflict
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	if len(resolved) != 2 {
		t.Errorf("expected 2 resolved, got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}
}

func TestResolveConflictsMissingFileInfo(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	// Test case: Local deleted, remote modified (should download with "recent" policy)
	decisions := []*cache.SyncDecision{
		{
			LocalPath:       "deleted_local.txt",
			RemotePath:      "deleted_local.txt",
			LocalInfo:       nil, // Deleted locally
			RemoteInfo:      &cache.FileInfo{Path: "deleted_local.txt", Size: 100},
			CachedInfo:      &cache.FileInfo{Path: "deleted_local.txt", Size: 50}, // Was different
			NeedsResolution: true,
		},
	}

	resolved, unresolved := resolver.ResolveConflicts(decisions)

	// With the new behavior, mod/del conflicts are resolved
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved (mod/del conflict), got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}

	// Should resolve to download (keep the modified remote)
	if len(resolved) > 0 && resolved[0].Action != cache.ActionDownload {
		t.Errorf("expected ActionDownload, got %s", resolved[0].Action)
	}
}

func TestGetSetPolicy(t *testing.T) {
	resolver, _ := NewConflictResolver("recent", zap.NewNop())

	if resolver.GetPolicy() != ConflictResolutionRecent {
		t.Errorf("expected policy 'recent', got '%s'", resolver.GetPolicy())
	}

	err := resolver.SetPolicy("local")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if resolver.GetPolicy() != ConflictResolutionLocal {
		t.Errorf("expected policy 'local', got '%s'", resolver.GetPolicy())
	}

	// Try invalid policy
	err = resolver.SetPolicy("invalid")
	if err == nil {
		t.Error("expected error for invalid policy")
	}

	// Policy should remain unchanged
	if resolver.GetPolicy() != ConflictResolutionLocal {
		t.Error("policy should not change on error")
	}
}

func TestCountConflicts(t *testing.T) {
	decisions := []*cache.SyncDecision{
		{NeedsResolution: true},
		{NeedsResolution: false},
		{NeedsResolution: true},
		{NeedsResolution: false},
		{NeedsResolution: true},
	}

	count := CountConflicts(decisions)

	if count != 3 {
		t.Errorf("expected 3 conflicts, got %d", count)
	}
}

func TestCountConflictsEmpty(t *testing.T) {
	decisions := []*cache.SyncDecision{}
	count := CountConflicts(decisions)

	if count != 0 {
		t.Errorf("expected 0 conflicts, got %d", count)
	}
}

func TestSeparateConflicts(t *testing.T) {
	decisions := []*cache.SyncDecision{
		{LocalPath: "conflict1.txt", NeedsResolution: true},
		{LocalPath: "normal1.txt", NeedsResolution: false},
		{LocalPath: "conflict2.txt", NeedsResolution: true},
		{LocalPath: "normal2.txt", NeedsResolution: false},
		{LocalPath: "conflict3.txt", NeedsResolution: true},
	}

	conflicts, normal := SeparateConflicts(decisions)

	if len(conflicts) != 3 {
		t.Errorf("expected 3 conflicts, got %d", len(conflicts))
	}
	if len(normal) != 2 {
		t.Errorf("expected 2 normal, got %d", len(normal))
	}

	// Verify conflicts
	for _, decision := range conflicts {
		if !decision.NeedsResolution {
			t.Error("conflict list contains non-conflict")
		}
	}

	// Verify normal
	for _, decision := range normal {
		if decision.NeedsResolution {
			t.Error("normal list contains conflict")
		}
	}
}

func TestSeparateConflictsEmpty(t *testing.T) {
	decisions := []*cache.SyncDecision{}

	conflicts, normal := SeparateConflicts(decisions)

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
	if len(normal) != 0 {
		t.Errorf("expected 0 normal, got %d", len(normal))
	}
}
