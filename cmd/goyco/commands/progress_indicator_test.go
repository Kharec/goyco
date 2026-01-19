package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockClock struct {
	mu  sync.RWMutex
	now time.Time
}

func newMockClock() *mockClock {
	return &mockClock{
		now: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (c *mockClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *mockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *mockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestNewProgressIndicator(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		description string
		expected    *ProgressIndicator
	}{
		{
			name:        "basic progress indicator",
			total:       100,
			description: "Test operation",
			expected: &ProgressIndicator{
				total:       100,
				current:     0,
				description: "Test operation",
				showETA:     true,
			},
		},
		{
			name:        "zero total",
			total:       0,
			description: "Empty operation",
			expected: &ProgressIndicator{
				total:       0,
				current:     0,
				description: "Empty operation",
				showETA:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := NewProgressIndicator(tt.total, tt.description)

			if pi.total != tt.expected.total {
				t.Errorf("expected total %d, got %d", tt.expected.total, pi.total)
			}
			if pi.current != tt.expected.current {
				t.Errorf("expected current %d, got %d", tt.expected.current, pi.current)
			}
			if pi.description != tt.expected.description {
				t.Errorf("expected description %q, got %q", tt.expected.description, pi.description)
			}
			if pi.showETA != tt.expected.showETA {
				t.Errorf("expected showETA %v, got %v", tt.expected.showETA, pi.showETA)
			}
			if pi.startTime.IsZero() {
				t.Error("expected startTime to be set")
			}
			if pi.lastUpdate.IsZero() {
				t.Error("expected lastUpdate to be set")
			}
		})
	}
}

func TestProgressIndicator_Update(t *testing.T) {
	clock := newMockClock()
	pi := newProgressIndicatorWithClock(10, "Test", clock)

	pi.Update(5)
	if pi.current != 5 {
		t.Errorf("expected current to be 5, got %d", pi.current)
	}

	originalLastUpdate := pi.lastUpdate
	clock.Advance(50 * time.Millisecond)
	pi.Update(6)
	if pi.current != 6 {
		t.Errorf("expected current to be 6, got %d", pi.current)
	}
	if !pi.lastUpdate.Equal(originalLastUpdate) {
		t.Error("expected lastUpdate to remain unchanged due to throttling")
	}

	clock.Advance(150 * time.Millisecond)
	lastUpdateBefore := pi.lastUpdate
	pi.Update(7)
	if pi.current != 7 {
		t.Errorf("expected current to be 7, got %d", pi.current)
	}
	if pi.lastUpdate.Equal(lastUpdateBefore) {
		t.Error("expected lastUpdate to be updated after throttling period")
	}
}

func TestProgressIndicator_Increment(t *testing.T) {
	pi := NewProgressIndicator(10, "Test")
	originalCurrent := pi.current

	pi.Increment()
	if pi.current != originalCurrent+1 {
		t.Errorf("expected current to be %d, got %d", originalCurrent+1, pi.current)
	}
}

func TestProgressIndicator_SetDescription(t *testing.T) {
	pi := NewProgressIndicator(10, "Original")
	newDesc := "New description"

	pi.SetDescription(newDesc)
	if pi.description != newDesc {
		t.Errorf("expected description %q, got %q", newDesc, pi.description)
	}
}

func TestProgressIndicator_Complete(t *testing.T) {
	pi := NewProgressIndicator(10, "Test")
	pi.current = 5

	output := captureOutput(func() {
		pi.Complete()
	})

	if pi.current != pi.total {
		t.Errorf("expected current to be %d, got %d", pi.total, pi.current)
	}

	if !strings.Contains(output, "Test") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "10/10") {
		t.Error("expected output to contain final count")
	}
	if !strings.Contains(output, "100.0%") {
		t.Error("expected output to contain 100%")
	}
}

func TestProgressIndicator_display(t *testing.T) {
	pi := NewProgressIndicator(10, "Test")
	pi.current = 3

	output := captureOutput(func() {
		pi.display()
	})

	if !strings.Contains(output, "Test") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "3/10") {
		t.Error("expected output to contain current/total")
	}
	if !strings.Contains(output, "30.0%") {
		t.Error("expected output to contain percentage")
	}
	if !strings.Contains(output, "[") && !strings.Contains(output, "]") {
		t.Error("expected output to contain progress bar")
	}
}

func TestNewSimpleProgressIndicator(t *testing.T) {
	clock := newMockClock()
	spi := newSimpleProgressIndicatorWithClock("Test operation", clock)

	if spi.description != "Test operation" {
		t.Errorf("expected description %q, got %q", "Test operation", spi.description)
	}
	if spi.current != 0 {
		t.Errorf("expected current 0, got %d", spi.current)
	}
	if spi.startTime.IsZero() {
		t.Error("expected startTime to be set")
	}
}

func TestSimpleProgressIndicator_Update(t *testing.T) {
	clock := newMockClock()
	spi := newSimpleProgressIndicatorWithClock("Test", clock)

	clock.Advance(2 * time.Second)

	output := captureOutput(func() {
		spi.Update(5)
	})

	if spi.current != 5 {
		t.Errorf("expected current 5, got %d", spi.current)
	}

	if !strings.Contains(output, "Test") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "5 items processed") {
		t.Error("expected output to contain item count")
	}
	if !strings.Contains(output, "2s") {
		t.Error("expected output to contain elapsed time (2s)")
	}
}

func TestSimpleProgressIndicator_Increment(t *testing.T) {
	clock := newMockClock()
	spi := newSimpleProgressIndicatorWithClock("Test", clock)
	originalCurrent := spi.current

	spi.Increment()
	if spi.current != originalCurrent+1 {
		t.Errorf("expected current to be %d, got %d", originalCurrent+1, spi.current)
	}
}

func TestSimpleProgressIndicator_Complete(t *testing.T) {
	clock := newMockClock()
	spi := newSimpleProgressIndicatorWithClock("Test", clock)
	spi.current = 5

	clock.Advance(5 * time.Second)

	output := captureOutput(func() {
		spi.Complete()
	})

	if !strings.Contains(output, "Test") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "Completed 5 items") {
		t.Error("expected output to contain completion message")
	}
	if !strings.Contains(output, "5s") {
		t.Error("expected output to contain elapsed time (5s)")
	}
}

func TestNewSpinner(t *testing.T) {
	spinner := NewSpinner("Loading")

	if spinner.message != "Loading" {
		t.Errorf("expected message %q, got %q", "Loading", spinner.message)
	}
	if spinner.index != 0 {
		t.Errorf("expected index 0, got %d", spinner.index)
	}
	if len(spinner.chars) != 4 {
		t.Errorf("expected 4 chars, got %d", len(spinner.chars))
	}
	if spinner.startTime.IsZero() {
		t.Error("expected startTime to be set")
	}
}

func TestSpinner_Spin(t *testing.T) {
	spinner := NewSpinner("Loading")
	originalIndex := spinner.index

	output := captureOutput(func() {
		spinner.Spin()
	})

	if spinner.index != (originalIndex+1)%len(spinner.chars) {
		t.Errorf("expected index to increment, got %d", spinner.index)
	}

	if !strings.Contains(output, "Loading") {
		t.Error("expected output to contain message")
	}
	if !strings.Contains(output, spinner.chars[originalIndex]) {
		t.Error("expected output to contain current char")
	}
}

func TestSpinner_Complete(t *testing.T) {
	spinner := NewSpinner("Loading")

	output := captureOutput(func() {
		spinner.Complete()
	})

	if !strings.Contains(output, "Loading") {
		t.Error("expected output to contain message")
	}
	if !strings.Contains(output, "✓") {
		t.Error("expected output to contain checkmark")
	}
}

func TestNewProgressTracker(t *testing.T) {
	pt := NewProgressTracker("Processing")

	if pt.description != "Processing" {
		t.Errorf("expected description %q, got %q", "Processing", pt.description)
	}
	if pt.current != 0 {
		t.Errorf("expected current 0, got %d", pt.current)
	}
	if pt.startTime.IsZero() {
		t.Error("expected startTime to be set")
	}
	if pt.lastUpdate.IsZero() {
		t.Error("expected lastUpdate to be set")
	}
}

func TestProgressTracker_Update(t *testing.T) {
	pt := NewProgressTracker("Processing")

	pt.Update(5)
	if pt.current != 5 {
		t.Errorf("expected current to be 5, got %d", pt.current)
	}

	originalLastUpdate := pt.lastUpdate
	pt.Update(6)
	if pt.current != 6 {
		t.Errorf("expected current to be 6, got %d", pt.current)
	}
	if !pt.lastUpdate.Equal(originalLastUpdate) {
		t.Error("expected lastUpdate to remain unchanged due to throttling")
	}

	time.Sleep(250 * time.Millisecond)
	lastUpdateBefore := pt.lastUpdate
	pt.Update(10)
	if pt.current != 10 {
		t.Errorf("expected current to be 10, got %d", pt.current)
	}
	if pt.lastUpdate.Equal(lastUpdateBefore) {
		t.Error("expected lastUpdate to be updated after throttling period")
	}
}

func TestProgressTracker_Increment(t *testing.T) {
	pt := NewProgressTracker("Processing")
	originalCurrent := pt.current

	pt.Increment()
	if pt.current != originalCurrent+1 {
		t.Errorf("expected current to be %d, got %d", originalCurrent+1, pt.current)
	}
}

func TestProgressTracker_Complete(t *testing.T) {
	pt := NewProgressTracker("Processing")
	pt.current = 10

	output := captureOutput(func() {
		pt.Complete()
	})

	if !strings.Contains(output, "Processing") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "Completed 10 items") {
		t.Error("expected output to contain completion message")
	}
	if !strings.Contains(output, "items/sec") {
		t.Error("expected output to contain rate information")
	}
}

func TestNewBatchProgressIndicator(t *testing.T) {
	bpi := NewBatchProgressIndicator(5, 10, "Batch processing")

	if bpi.totalBatches != 5 {
		t.Errorf("expected totalBatches 5, got %d", bpi.totalBatches)
	}
	if bpi.currentBatch != 0 {
		t.Errorf("expected currentBatch 0, got %d", bpi.currentBatch)
	}
	if bpi.batchSize != 10 {
		t.Errorf("expected batchSize 10, got %d", bpi.batchSize)
	}
	if bpi.description != "Batch processing" {
		t.Errorf("expected description %q, got %q", "Batch processing", bpi.description)
	}
	if bpi.startTime.IsZero() {
		t.Error("expected startTime to be set")
	}
}

func TestBatchProgressIndicator_UpdateBatch(t *testing.T) {
	bpi := NewBatchProgressIndicator(5, 10, "Batch processing")

	output := captureOutput(func() {
		bpi.UpdateBatch(2)
	})

	if bpi.currentBatch != 2 {
		t.Errorf("expected currentBatch 2, got %d", bpi.currentBatch)
	}

	if !strings.Contains(output, "Batch processing") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "Batch 2/5") {
		t.Error("expected output to contain batch progress")
	}
	if !strings.Contains(output, "(20 items)") {
		t.Error("expected output to contain item count")
	}
}

func TestBatchProgressIndicator_Complete(t *testing.T) {
	bpi := NewBatchProgressIndicator(5, 10, "Batch processing")

	output := captureOutput(func() {
		bpi.Complete()
	})

	if !strings.Contains(output, "Batch processing") {
		t.Error("expected output to contain description")
	}
	if !strings.Contains(output, "Completed 5 batches") {
		t.Error("expected output to contain completion message")
	}
	if !strings.Contains(output, "(50 items)") {
		t.Error("expected output to contain total items")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "minutes",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2.5m",
		},
		{
			name:     "hours",
			duration: 1*time.Hour + 30*time.Minute,
			expected: "1.5h",
		},
		{
			name:     "zero duration",
			duration: 0,
			expected: "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProgressIndicator_Concurrency(t *testing.T) {
	pi := NewProgressIndicator(100, "Concurrent test")
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				pi.Increment()
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if pi.current != 100 {
		t.Errorf("expected current to be exactly 100, got %d", pi.current)
	}
}

func TestProgressIndicator_EdgeCases(t *testing.T) {
	t.Run("zero total constructor", func(t *testing.T) {
		pi := NewProgressIndicator(0, "Zero total")
		if pi.total != 0 {
			t.Errorf("expected total 0, got %d", pi.total)
		}
		if pi.current != 0 {
			t.Errorf("expected current 0, got %d", pi.current)
		}
	})

	t.Run("negative current", func(t *testing.T) {
		pi := NewProgressIndicator(10, "Negative test")
		pi.current = -1
		if pi.current != -1 {
			t.Errorf("expected current -1, got %d", pi.current)
		}
	})

	t.Run("current greater than total", func(t *testing.T) {
		pi := NewProgressIndicator(10, "Overflow test")
		pi.current = 15
		if pi.current != 15 {
			t.Errorf("expected current 15, got %d", pi.current)
		}
	})
}
