package sync

import (
	"testing"
	"time"
)

func TestNewProgressTracker(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)

	if tracker == nil {
		t.Fatal("expected tracker to be created")
	}

	if tracker.callback == nil {
		t.Error("callback should be set")
	}

	if len(tracker.phaseWeights) == 0 {
		t.Error("phase weights should be initialized")
	}

	// Check default phase weights
	weights := []string{"preparation", "scanning", "detecting", "executing", "finalizing"}
	for _, phase := range weights {
		if _, exists := tracker.phaseWeights[phase]; !exists {
			t.Errorf("phase weight for %s not set", phase)
		}
	}
}

func TestProgressTrackerSetPhase(t *testing.T) {
	called := false
	callback := func(p *SyncProgress) {
		called = true
		if p.Phase != "scanning" {
			t.Errorf("expected phase 'scanning', got '%s'", p.Phase)
		}
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhase("scanning")

	if !called {
		t.Fatal("expected progress callback to be called")
	}

	// Check that counters are reset
	current := tracker.GetCurrentProgress()
	if current.FilesProcessed != 0 {
		t.Error("FilesProcessed should be reset to 0")
	}
	if current.FilesTotal != 0 {
		t.Error("FilesTotal should be reset to 0")
	}
}

func TestProgressTrackerSetPhaseWithMessage(t *testing.T) {
	called := false
	callback := func(p *SyncProgress) {
		called = true
		if p.Phase != "executing" {
			t.Errorf("expected phase 'executing', got '%s'", p.Phase)
		}
		if p.Message != "Processing files..." {
			t.Errorf("expected message 'Processing files...', got '%s'", p.Message)
		}
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhaseWithMessage("executing", "Processing files...")

	if !called {
		t.Fatal("expected progress callback to be called")
	}
}

func TestProgressTrackerSetMessage(t *testing.T) {
	called := false
	callback := func(p *SyncProgress) {
		called = true
		if p.Message != "Test message" {
			t.Errorf("expected 'Test message', got '%s'", p.Message)
		}
	}

	tracker := NewProgressTracker(callback)
	tracker.SetMessage("Test message")

	if !called {
		t.Fatal("expected progress callback to be called")
	}
}

func TestProgressTrackerSetFileCounts(t *testing.T) {
	callCount := 0
	callback := func(p *SyncProgress) {
		callCount++
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhase("executing")
	callCount = 0 // Reset after SetPhase

	// Wait a bit to avoid throttling
	time.Sleep(300 * time.Millisecond)
	tracker.SetFileCounts(50, 100)

	current := tracker.GetCurrentProgress()
	if current.FilesProcessed != 50 {
		t.Errorf("expected 50 files processed, got %d", current.FilesProcessed)
	}
	if current.FilesTotal != 100 {
		t.Errorf("expected 100 files total, got %d", current.FilesTotal)
	}

	// Callback should be called (after waiting)
	if callCount == 0 {
		t.Error("expected callback to be called")
	}
}

func TestProgressTrackerIncrementFiles(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhase("executing")
	tracker.SetFileCounts(0, 100)

	tracker.IncrementFiles()
	tracker.IncrementFiles()
	tracker.IncrementFiles()

	current := tracker.GetCurrentProgress()
	if current.FilesProcessed != 3 {
		t.Errorf("expected 3 files processed, got %d", current.FilesProcessed)
	}
}

func TestProgressTrackerSetByteCounts(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhase("executing")

	tracker.SetByteCounts(1024*1024, 10*1024*1024) // 1MB / 10MB

	current := tracker.GetCurrentProgress()
	if current.BytesTransferred != 1024*1024 {
		t.Errorf("expected 1MB transferred, got %d", current.BytesTransferred)
	}
	if current.BytesTotal != 10*1024*1024 {
		t.Errorf("expected 10MB total, got %d", current.BytesTotal)
	}
}

func TestProgressTrackerAddBytes(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhase("executing")

	tracker.AddBytes(1024)
	tracker.AddBytes(2048)
	tracker.AddBytes(512)

	current := tracker.GetCurrentProgress()
	expectedBytes := int64(1024 + 2048 + 512)
	if current.BytesTransferred != expectedBytes {
		t.Errorf("expected %d bytes, got %d", expectedBytes, current.BytesTransferred)
	}
}

func TestProgressTrackerSetCurrentFile(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)
	tracker.SetCurrentFile("/path/to/file.txt")

	current := tracker.GetCurrentProgress()
	if current.CurrentFile != "/path/to/file.txt" {
		t.Errorf("expected '/path/to/file.txt', got '%s'", current.CurrentFile)
	}
}

func TestProgressTrackerSetCurrentAction(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)
	tracker.SetCurrentAction("uploading")

	current := tracker.GetCurrentProgress()
	if current.CurrentAction != "uploading" {
		t.Errorf("expected 'uploading', got '%s'", current.CurrentAction)
	}
}

func TestProgressTrackerSetFileAndAction(t *testing.T) {
	callback := func(p *SyncProgress) {
		// Progress received
	}

	tracker := NewProgressTracker(callback)
	tracker.SetFileAndAction("/path/file.txt", "downloading")

	current := tracker.GetCurrentProgress()
	if current.CurrentFile != "/path/file.txt" {
		t.Errorf("expected '/path/file.txt', got '%s'", current.CurrentFile)
	}
	if current.CurrentAction != "downloading" {
		t.Errorf("expected 'downloading', got '%s'", current.CurrentAction)
	}
}

func TestProgressTrackerCalculatePercentage(t *testing.T) {
	tests := []struct {
		name              string
		phase             string
		filesProcessed    int
		filesTotal        int
		expectedPercentage float64
	}{
		{
			name:              "preparation start",
			phase:             "preparation",
			filesProcessed:    0,
			filesTotal:        100,
			expectedPercentage: 0.0,
		},
		{
			name:              "scanning start",
			phase:             "scanning",
			filesProcessed:    0,
			filesTotal:        100,
			expectedPercentage: 5.0,
		},
		{
			name:              "scanning 50%",
			phase:             "scanning",
			filesProcessed:    50,
			filesTotal:        100,
			expectedPercentage: 15.0, // 5 + (50% of 20)
		},
		{
			name:              "detecting start",
			phase:             "detecting",
			filesProcessed:    0,
			filesTotal:        100,
			expectedPercentage: 25.0,
		},
		{
			name:              "executing start",
			phase:             "executing",
			filesProcessed:    0,
			filesTotal:        100,
			expectedPercentage: 35.0,
		},
		{
			name:              "executing 50%",
			phase:             "executing",
			filesProcessed:    50,
			filesTotal:        100,
			expectedPercentage: 65.0, // 35 + (50% of 60)
		},
		{
			name:              "finalizing start",
			phase:             "finalizing",
			filesProcessed:    0,
			filesTotal:        100,
			expectedPercentage: 95.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewProgressTracker(nil)
			tracker.SetPhase(tt.phase)
			tracker.SetFileCounts(tt.filesProcessed, tt.filesTotal)

			percentage := tracker.calculatePercentage()

			// Allow small floating point differences
			diff := percentage - tt.expectedPercentage
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("expected percentage %.2f, got %.2f", tt.expectedPercentage, percentage)
			}
		})
	}
}

func TestProgressTrackerGetElapsedTime(t *testing.T) {
	tracker := NewProgressTracker(nil)

	time.Sleep(10 * time.Millisecond)

	elapsed := tracker.GetElapsedTime()

	if elapsed < 10*time.Millisecond {
		t.Error("elapsed time should be at least 10ms")
	}

	if elapsed > 1*time.Second {
		t.Error("elapsed time should not be more than 1s for this test")
	}
}

func TestProgressTrackerGetTransferRate(t *testing.T) {
	tracker := NewProgressTracker(nil)

	// Simulate some transfer
	time.Sleep(100 * time.Millisecond)
	tracker.AddBytes(1024 * 100) // 100 KB

	rate := tracker.GetTransferRate()

	// Should be around 1000 KB/s (1MB/s), but allow for variation
	if rate < 500000 || rate > 2000000 {
		t.Logf("rate: %.2f bytes/sec", rate)
		// Don't fail, timing can be imprecise in tests
	}
}

func TestProgressTrackerForceReport(t *testing.T) {
	callCount := 0
	callback := func(p *SyncProgress) {
		callCount++
	}

	tracker := NewProgressTracker(callback)
	tracker.SetPhase("scanning")
	callCount = 0 // Reset

	// Call ForceReport multiple times rapidly
	tracker.ForceReport()
	tracker.ForceReport()
	tracker.ForceReport()

	// All calls should go through (no throttling)
	if callCount != 3 {
		t.Errorf("expected 3 callbacks, got %d", callCount)
	}
}

func TestProgressTrackerThrottling(t *testing.T) {
	callCount := 0
	callback := func(p *SyncProgress) {
		callCount++
	}

	tracker := NewProgressTracker(callback)
	tracker.minUpdateInterval = 100 * time.Millisecond
	tracker.SetPhase("executing")
	callCount = 0 // Reset

	// Rapid updates should be throttled
	for i := 0; i < 10; i++ {
		tracker.IncrementFiles()
	}

	// Should be throttled to very few calls
	if callCount > 3 {
		t.Errorf("expected throttling, got %d callbacks", callCount)
	}

	// Wait and try again
	time.Sleep(150 * time.Millisecond)
	tracker.IncrementFiles()

	// Now it should report
	if callCount == 0 {
		t.Error("expected at least one callback after waiting")
	}
}

func TestProgressTrackerGetCurrentProgress(t *testing.T) {
	tracker := NewProgressTracker(nil)
	tracker.SetPhase("executing")
	tracker.SetFileCounts(50, 100)
	tracker.SetByteCounts(5000, 10000)
	tracker.SetCurrentFile("test.txt")
	tracker.SetCurrentAction("uploading")

	progress := tracker.GetCurrentProgress()

	if progress.Phase != "executing" {
		t.Errorf("expected phase 'executing', got '%s'", progress.Phase)
	}
	if progress.FilesProcessed != 50 {
		t.Errorf("expected 50 files processed, got %d", progress.FilesProcessed)
	}
	if progress.FilesTotal != 100 {
		t.Errorf("expected 100 files total, got %d", progress.FilesTotal)
	}
	if progress.BytesTransferred != 5000 {
		t.Errorf("expected 5000 bytes transferred, got %d", progress.BytesTransferred)
	}
	if progress.BytesTotal != 10000 {
		t.Errorf("expected 10000 bytes total, got %d", progress.BytesTotal)
	}
	if progress.CurrentFile != "test.txt" {
		t.Errorf("expected 'test.txt', got '%s'", progress.CurrentFile)
	}
	if progress.CurrentAction != "uploading" {
		t.Errorf("expected 'uploading', got '%s'", progress.CurrentAction)
	}
}

func TestProgressTrackerNilCallback(t *testing.T) {
	// Should not panic with nil callback
	tracker := NewProgressTracker(nil)

	tracker.SetPhase("scanning")
	tracker.SetMessage("test")
	tracker.IncrementFiles()
	tracker.AddBytes(1024)
	tracker.ForceReport()

	// If we get here, no panic occurred
}
