package cache

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewChangeDetector(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())
	cd := NewChangeDetector(cm, zap.NewNop())

	if cd == nil {
		t.Fatal("expected change detector but got nil")
	}
	if cd.cache != cm {
		t.Error("change detector has wrong cache reference")
	}
}

func TestChangeDetector_DetermineSyncAction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())
	cd := NewChangeDetector(cm, zap.NewNop())

	jobID := int64(1)
	localPath := "/test/file.txt"
	remotePath := "/remote/file.txt"
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name           string
		local          *FileInfo
		remote         *FileInfo
		cached         *FileInfo
		expectedAction SyncAction
	}{
		{
			name:           "new local file",
			local:          &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remote:         nil,
			cached:         nil,
			expectedAction: ActionUpload,
		},
		{
			name:           "new remote file",
			local:          nil,
			remote:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			cached:         nil,
			expectedAction: ActionDownload,
		},
		{
			name:           "both new with same content",
			local:          &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remote:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			cached:         nil,
			expectedAction: ActionNone,
		},
		{
			name:           "both new with different content",
			local:          &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remote:         &FileInfo{Size: 200, MTime: now, Hash: "hash2"},
			cached:         nil,
			expectedAction: ActionConflict,
		},
		{
			name:           "local modified",
			local:          &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			remote:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionUpload,
		},
		{
			name:           "remote modified",
			local:          &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remote:         &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionDownload,
		},
		{
			name:           "both modified to same content",
			local:          &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			remote:         &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionNone,
		},
		{
			name:           "both modified to different content",
			local:          &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			remote:         &FileInfo{Size: 300, MTime: now.Add(time.Hour), Hash: "hash3"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionConflict,
		},
		{
			name:           "deleted locally",
			local:          nil,
			remote:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionDeleteRemote,
		},
		{
			name:           "deleted remotely",
			local:          &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remote:         nil,
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionDeleteLocal,
		},
		{
			name:           "deleted locally but modified remotely",
			local:          nil,
			remote:         &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionConflict,
		},
		{
			name:           "modified locally but deleted remotely",
			local:          &FileInfo{Size: 200, MTime: now.Add(time.Hour), Hash: "hash2"},
			remote:         nil,
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionConflict,
		},
		{
			name:           "no changes",
			local:          &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remote:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			cached:         &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			expectedAction: ActionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup cached state
			if tt.cached != nil {
				err := cm.UpdateCache(jobID, localPath, remotePath, tt.cached)
				if err != nil {
					t.Fatalf("failed to setup cache: %v", err)
				}
			} else {
				cm.RemoveFromCache(jobID, localPath)
			}

			// Determine sync action
			decision, err := cd.DetermineSyncAction(jobID, localPath, remotePath, tt.local, tt.remote)
			if err != nil {
				t.Fatalf("failed to determine sync action: %v", err)
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("expected action %s, got %s (reason: %s)",
					tt.expectedAction, decision.Action, decision.Reason)
			}

			// Cleanup
			cm.RemoveFromCache(jobID, localPath)
		})
	}
}

func TestChangeDetector_ResolveConflict(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())
	cd := NewChangeDetector(cm, zap.NewNop())

	now := time.Now().Truncate(time.Second)
	localNewer := now.Add(time.Hour)
	remoteNewer := now.Add(2 * time.Hour)

	tests := []struct {
		name           string
		resolution     string
		localInfo      *FileInfo
		remoteInfo     *FileInfo
		expectedAction SyncAction
		expectErr      bool
	}{
		{
			name:           "resolve to local",
			resolution:     "local",
			localInfo:      &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remoteInfo:     &FileInfo{Size: 200, MTime: now, Hash: "hash2"},
			expectedAction: ActionUpload,
			expectErr:      false,
		},
		{
			name:           "resolve to remote",
			resolution:     "remote",
			localInfo:      &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remoteInfo:     &FileInfo{Size: 200, MTime: now, Hash: "hash2"},
			expectedAction: ActionDownload,
			expectErr:      false,
		},
		{
			name:           "resolve to recent (local newer)",
			resolution:     "recent",
			localInfo:      &FileInfo{Size: 100, MTime: localNewer, Hash: "hash1"},
			remoteInfo:     &FileInfo{Size: 200, MTime: now, Hash: "hash2"},
			expectedAction: ActionUpload,
			expectErr:      false,
		},
		{
			name:           "resolve to recent (remote newer)",
			resolution:     "recent",
			localInfo:      &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remoteInfo:     &FileInfo{Size: 200, MTime: remoteNewer, Hash: "hash2"},
			expectedAction: ActionDownload,
			expectErr:      false,
		},
		{
			name:       "invalid resolution strategy",
			resolution: "invalid",
			localInfo:  &FileInfo{Size: 100, MTime: now, Hash: "hash1"},
			remoteInfo: &FileInfo{Size: 200, MTime: now, Hash: "hash2"},
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create conflict decision
			decision := &SyncDecision{
				LocalPath:       "/test/file.txt",
				RemotePath:      "/remote/file.txt",
				Action:          ActionConflict,
				LocalInfo:       tt.localInfo,
				RemoteInfo:      tt.remoteInfo,
				NeedsResolution: true,
			}

			// Resolve conflict
			err := cd.ResolveConflict(decision, tt.resolution)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("expected action %s, got %s", tt.expectedAction, decision.Action)
			}

			if decision.NeedsResolution {
				t.Error("decision should not need resolution after resolving")
			}
		})
	}
}

func TestChangeDetector_ResolveNonConflict(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())
	cd := NewChangeDetector(cm, zap.NewNop())

	// Try to resolve a non-conflict decision
	decision := &SyncDecision{
		Action: ActionUpload,
	}

	err := cd.ResolveConflict(decision, "local")
	if err == nil {
		t.Error("expected error when resolving non-conflict decision")
	}
}

func TestChangeDetector_BatchDetermineSyncActions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())
	cd := NewChangeDetector(cm, zap.NewNop())

	jobID := int64(1)
	now := time.Now().Truncate(time.Second)

	// Setup: Add some files to cache
	cachedFiles := map[string]*FileInfo{
		"/file1.txt": {Size: 100, MTime: now, Hash: "hash1"},
		"/file2.txt": {Size: 200, MTime: now, Hash: "hash2"},
		"/file3.txt": {Size: 300, MTime: now, Hash: "hash3"},
	}
	remotePaths := make(map[string]string)
	for path := range cachedFiles {
		remotePaths[path] = path
	}
	err := cm.UpdateCacheBatch(jobID, cachedFiles, remotePaths)
	if err != nil {
		t.Fatalf("failed to setup cache: %v", err)
	}

	// Local files (file1 modified, file2 unchanged, file3 deleted, file4 new)
	localFiles := map[string]*FileInfo{
		"/file1.txt": {Size: 150, MTime: now.Add(time.Hour), Hash: "hash1_new"},
		"/file2.txt": {Size: 200, MTime: now, Hash: "hash2"},
		"/file4.txt": {Size: 400, MTime: now, Hash: "hash4"},
	}

	// Remote files (file1 unchanged, file2 modified, file3 unchanged, file5 new)
	remoteFiles := map[string]*FileInfo{
		"/file1.txt": {Size: 100, MTime: now, Hash: "hash1"},
		"/file2.txt": {Size: 250, MTime: now.Add(time.Hour), Hash: "hash2_new"},
		"/file3.txt": {Size: 300, MTime: now, Hash: "hash3"},
		"/file5.txt": {Size: 500, MTime: now, Hash: "hash5"},
	}

	// Determine sync actions
	decisions, err := cd.BatchDetermineSyncActions(jobID, localFiles, remoteFiles)
	if err != nil {
		t.Fatalf("failed to batch determine sync actions: %v", err)
	}

	// Expected actions:
	// file1: local modified -> upload
	// file2: remote modified -> download
	// file3: local deleted -> delete remote
	// file4: new local -> upload
	// file5: new remote -> download
	// Total: 5 actions

	if len(decisions) != 5 {
		t.Errorf("expected 5 decisions, got %d", len(decisions))
	}

	// Verify specific actions
	actionMap := make(map[string]SyncAction)
	for _, d := range decisions {
		actionMap[d.LocalPath] = d.Action
	}

	expectedActions := map[string]SyncAction{
		"/file1.txt": ActionUpload,
		"/file2.txt": ActionDownload,
		"/file3.txt": ActionDeleteRemote,
		"/file4.txt": ActionUpload,
		"/file5.txt": ActionDownload,
	}

	for path, expectedAction := range expectedActions {
		actualAction, ok := actionMap[path]
		if !ok {
			t.Errorf("missing decision for %s", path)
			continue
		}
		if actualAction != expectedAction {
			t.Errorf("%s: expected action %s, got %s", path, expectedAction, actualAction)
		}
	}
}
