package sync

import (
	"sync"
	"time"
)

// ProgressTracker tracks progress across multiple phases
type ProgressTracker struct {
	mu sync.RWMutex

	// Callback to report progress
	callback ProgressCallback

	// Current state
	currentPhase      string
	currentFile       string
	currentAction     string
	message           string

	// File progress
	filesProcessed int
	filesTotal     int

	// Byte progress
	bytesTransferred int64
	bytesTotal       int64

	// Phase weights for percentage calculation
	phaseWeights map[string]PhaseWeight

	// Start time for rate calculation
	startTime time.Time
	lastUpdate time.Time

	// Rate limiting for callbacks
	minUpdateInterval time.Duration
}

// PhaseWeight defines the weight and range for each phase
type PhaseWeight struct {
	StartPercentage float64
	EndPercentage   float64
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(callback ProgressCallback) *ProgressTracker {
	// Default phase weights (can be customized)
	weights := map[string]PhaseWeight{
		"preparation": {StartPercentage: 0, EndPercentage: 5},
		"scanning":    {StartPercentage: 5, EndPercentage: 25},
		"detecting":   {StartPercentage: 25, EndPercentage: 35},
		"executing":   {StartPercentage: 35, EndPercentage: 95},
		"finalizing":  {StartPercentage: 95, EndPercentage: 100},
	}

	return &ProgressTracker{
		callback:          callback,
		phaseWeights:      weights,
		startTime:         time.Now(),
		lastUpdate:        time.Now(),
		minUpdateInterval: 250 * time.Millisecond, // Update at most every 250ms
	}
}

// SetPhase sets the current phase
func (pt *ProgressTracker) SetPhase(phase string) {
	pt.mu.Lock()
	pt.currentPhase = phase
	pt.filesProcessed = 0
	pt.filesTotal = 0
	pt.bytesTransferred = 0
	pt.bytesTotal = 0
	pt.mu.Unlock()

	pt.report()
}

// SetPhaseWithMessage sets the current phase with a message
func (pt *ProgressTracker) SetPhaseWithMessage(phase, message string) {
	pt.mu.Lock()
	pt.currentPhase = phase
	pt.message = message
	pt.filesProcessed = 0
	pt.filesTotal = 0
	pt.bytesTransferred = 0
	pt.bytesTotal = 0
	pt.mu.Unlock()

	pt.report()
}

// SetMessage sets a status message
func (pt *ProgressTracker) SetMessage(message string) {
	pt.mu.Lock()
	pt.message = message
	pt.mu.Unlock()

	pt.report()
}

// SetFileCounts sets the file counts
func (pt *ProgressTracker) SetFileCounts(processed, total int) {
	pt.mu.Lock()
	pt.filesProcessed = processed
	pt.filesTotal = total
	pt.mu.Unlock()

	pt.reportThrottled()
}

// IncrementFiles increments the files processed counter
func (pt *ProgressTracker) IncrementFiles() {
	pt.mu.Lock()
	pt.filesProcessed++
	pt.mu.Unlock()

	pt.reportThrottled()
}

// SetByteCounts sets the byte counts
func (pt *ProgressTracker) SetByteCounts(transferred, total int64) {
	pt.mu.Lock()
	pt.bytesTransferred = transferred
	pt.bytesTotal = total
	pt.mu.Unlock()

	pt.reportThrottled()
}

// AddBytes adds to the bytes transferred counter
func (pt *ProgressTracker) AddBytes(bytes int64) {
	pt.mu.Lock()
	pt.bytesTransferred += bytes
	pt.mu.Unlock()

	pt.reportThrottled()
}

// SetCurrentFile sets the current file being processed
func (pt *ProgressTracker) SetCurrentFile(file string) {
	pt.mu.Lock()
	pt.currentFile = file
	pt.mu.Unlock()

	pt.reportThrottled()
}

// SetCurrentAction sets the current action
func (pt *ProgressTracker) SetCurrentAction(action string) {
	pt.mu.Lock()
	pt.currentAction = action
	pt.mu.Unlock()

	pt.reportThrottled()
}

// SetFileAndAction sets both file and action atomically
func (pt *ProgressTracker) SetFileAndAction(file, action string) {
	pt.mu.Lock()
	pt.currentFile = file
	pt.currentAction = action
	pt.mu.Unlock()

	pt.reportThrottled()
}

// calculatePercentage calculates the overall percentage based on current phase and progress
func (pt *ProgressTracker) calculatePercentage() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	weight, exists := pt.phaseWeights[pt.currentPhase]
	if !exists {
		// Unknown phase, return 0
		return 0.0
	}

	// Start with the phase's start percentage
	percentage := weight.StartPercentage

	// Add progress within the phase
	phaseRange := weight.EndPercentage - weight.StartPercentage

	// Calculate progress based on files or bytes
	var progress float64
	if pt.filesTotal > 0 {
		progress = float64(pt.filesProcessed) / float64(pt.filesTotal)
	} else if pt.bytesTotal > 0 {
		progress = float64(pt.bytesTransferred) / float64(pt.bytesTotal)
	}

	// Clamp progress to [0, 1]
	if progress > 1.0 {
		progress = 1.0
	}

	percentage += progress * phaseRange

	return percentage
}

// report sends a progress update (always)
func (pt *ProgressTracker) report() {
	if pt.callback == nil {
		return
	}

	pt.mu.RLock()
	progress := &SyncProgress{
		Phase:            pt.currentPhase,
		CurrentFile:      pt.currentFile,
		FilesProcessed:   pt.filesProcessed,
		FilesTotal:       pt.filesTotal,
		BytesTransferred: pt.bytesTransferred,
		BytesTotal:       pt.bytesTotal,
		CurrentAction:    pt.currentAction,
		Percentage:       pt.calculatePercentage(),
		Message:          pt.message,
	}
	pt.lastUpdate = time.Now()
	pt.mu.RUnlock()

	pt.callback(progress)
}

// reportThrottled sends a progress update (throttled by minUpdateInterval)
func (pt *ProgressTracker) reportThrottled() {
	if pt.callback == nil {
		return
	}

	pt.mu.RLock()
	timeSinceLastUpdate := time.Since(pt.lastUpdate)
	pt.mu.RUnlock()

	if timeSinceLastUpdate < pt.minUpdateInterval {
		return
	}

	pt.report()
}

// ForceReport forces an immediate progress update (bypasses throttling)
func (pt *ProgressTracker) ForceReport() {
	pt.report()
}

// GetCurrentProgress returns the current progress without triggering a callback
func (pt *ProgressTracker) GetCurrentProgress() *SyncProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return &SyncProgress{
		Phase:            pt.currentPhase,
		CurrentFile:      pt.currentFile,
		FilesProcessed:   pt.filesProcessed,
		FilesTotal:       pt.filesTotal,
		BytesTransferred: pt.bytesTransferred,
		BytesTotal:       pt.bytesTotal,
		CurrentAction:    pt.currentAction,
		Percentage:       pt.calculatePercentage(),
		Message:          pt.message,
	}
}

// GetElapsedTime returns the time since the tracker was created
func (pt *ProgressTracker) GetElapsedTime() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return time.Since(pt.startTime)
}

// GetTransferRate returns the current transfer rate in bytes/sec
func (pt *ProgressTracker) GetTransferRate() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	elapsed := time.Since(pt.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}

	return float64(pt.bytesTransferred) / elapsed
}

// GetEstimatedTimeRemaining estimates time remaining based on current rate
func (pt *ProgressTracker) GetEstimatedTimeRemaining() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.bytesTotal == 0 || pt.bytesTransferred == 0 {
		return 0
	}

	rate := pt.GetTransferRate()
	if rate == 0 {
		return 0
	}

	remaining := pt.bytesTotal - pt.bytesTransferred
	secondsRemaining := float64(remaining) / rate

	return time.Duration(secondsRemaining * float64(time.Second))
}
