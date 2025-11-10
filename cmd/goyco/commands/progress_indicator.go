package commands

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (c *realClock) Now() time.Time {
	return time.Now()
}

type ProgressIndicator struct {
	total       int
	current     int
	startTime   time.Time
	lastUpdate  time.Time
	description string
	showETA     bool
	mu          sync.Mutex
	clock       clock
}

func NewProgressIndicator(total int, description string) *ProgressIndicator {
	return &ProgressIndicator{
		total:       total,
		current:     0,
		startTime:   time.Now(),
		lastUpdate:  time.Now(),
		description: description,
		showETA:     true,
		clock:       &realClock{},
	}
}

func newProgressIndicatorWithClock(total int, description string, c clock) *ProgressIndicator {
	now := c.Now()
	return &ProgressIndicator{
		total:       total,
		current:     0,
		startTime:   now,
		lastUpdate:  now,
		description: description,
		showETA:     true,
		clock:       c,
	}
}

func (p *ProgressIndicator) Update(current int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = current
	now := p.clock.Now()

	if now.Sub(p.lastUpdate) < 100*time.Millisecond {
		return
	}

	p.lastUpdate = now
	p.display()
}

func (p *ProgressIndicator) Increment() {
	p.mu.Lock()
	p.current++
	current := p.current
	now := p.clock.Now()

	shouldUpdate := now.Sub(p.lastUpdate) >= 100*time.Millisecond
	if shouldUpdate {
		p.lastUpdate = now
	}
	p.mu.Unlock()

	if shouldUpdate {
		p.displayWithValue(current)
	}
}

func (p *ProgressIndicator) SetDescription(description string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.description = description
}

func (p *ProgressIndicator) Current() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current
}

func (p *ProgressIndicator) Complete() {
	p.mu.Lock()
	p.current = p.total
	p.mu.Unlock()
	p.display()
	fmt.Println()
}

func (p *ProgressIndicator) display() {
	p.mu.Lock()
	current := p.current
	p.mu.Unlock()
	p.displayWithValue(current)
}

func (p *ProgressIndicator) displayWithValue(current int) {
	p.mu.Lock()
	total := p.total
	description := p.description
	showETA := p.showETA
	startTime := p.startTime
	now := p.clock.Now()
	p.mu.Unlock()

	percentage := float64(current) / float64(total) * 100

	barWidth := 50
	filled := int(float64(barWidth) * percentage / 100)
	bar := strings.Repeat("=", filled) + strings.Repeat("-", barWidth-filled)

	var etaStr string
	if showETA && current > 0 {
		elapsed := now.Sub(startTime)
		rate := float64(current) / elapsed.Seconds()
		if rate > 0 {
			remaining := float64(total-current) / rate
			eta := time.Duration(remaining) * time.Second
			etaStr = fmt.Sprintf(" ETA: %s", formatDuration(eta))
		}
	}

	elapsed := now.Sub(startTime)
	elapsedStr := formatDuration(elapsed)

	fmt.Printf("\r%s [%s] %d/%d (%.1f%%) %s%s",
		description, bar, current, total, percentage, elapsedStr, etaStr)

	_ = os.Stdout.Sync()
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

type SimpleProgressIndicator struct {
	description string
	startTime   time.Time
	current     int
	clock       clock
}

func NewSimpleProgressIndicator(description string) *SimpleProgressIndicator {
	now := time.Now()
	return &SimpleProgressIndicator{
		description: description,
		startTime:   now,
		current:     0,
		clock:       &realClock{},
	}
}

func newSimpleProgressIndicatorWithClock(description string, c clock) *SimpleProgressIndicator {
	now := c.Now()
	return &SimpleProgressIndicator{
		description: description,
		startTime:   now,
		current:     0,
		clock:       c,
	}
}

func (s *SimpleProgressIndicator) Update(current int) {
	s.current = current
	elapsed := s.clock.Now().Sub(s.startTime)
	fmt.Printf("\r%s: %d items processed in %s",
		s.description, s.current, formatDuration(elapsed))
	_ = os.Stdout.Sync()
}

func (s *SimpleProgressIndicator) Increment() {
	s.Update(s.current + 1)
}

func (s *SimpleProgressIndicator) Complete() {
	elapsed := s.clock.Now().Sub(s.startTime)
	fmt.Printf("\r%s: Completed %d items in %s\n",
		s.description, s.current, formatDuration(elapsed))
}

type Spinner struct {
	chars     []string
	index     int
	message   string
	startTime time.Time
}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		chars:     []string{"|", "/", "-", "\\"},
		index:     0,
		message:   message,
		startTime: time.Now(),
	}
}

func (s *Spinner) Spin() {
	elapsed := time.Since(s.startTime)
	fmt.Printf("\r%s %s (%s)", s.message, s.chars[s.index], formatDuration(elapsed))
	s.index = (s.index + 1) % len(s.chars)
	_ = os.Stdout.Sync()
}

func (s *Spinner) Complete() {
	elapsed := time.Since(s.startTime)
	fmt.Printf("\r%s ✓ (%s)\n", s.message, formatDuration(elapsed))
}

type ProgressTracker struct {
	description string
	startTime   time.Time
	current     int
	lastUpdate  time.Time
}

func NewProgressTracker(description string) *ProgressTracker {
	return &ProgressTracker{
		description: description,
		startTime:   time.Now(),
		current:     0,
		lastUpdate:  time.Now(),
	}
}

func (pt *ProgressTracker) Update(current int) {
	pt.current = current
	now := time.Now()

	if now.Sub(pt.lastUpdate) < 200*time.Millisecond {
		return
	}

	pt.lastUpdate = now
	elapsed := time.Since(pt.startTime)
	rate := float64(current) / elapsed.Seconds()

	fmt.Printf("\r%s: %d items processed (%.1f items/sec)",
		pt.description, current, rate)
	_ = os.Stdout.Sync()
}

func (pt *ProgressTracker) Increment() {
	pt.Update(pt.current + 1)
}

func (pt *ProgressTracker) Complete() {
	elapsed := time.Since(pt.startTime)
	rate := float64(pt.current) / elapsed.Seconds()
	fmt.Printf("\r%s: Completed %d items in %s (%.1f items/sec)\n",
		pt.description, pt.current, formatDuration(elapsed), rate)
}

type BatchProgressIndicator struct {
	totalBatches int
	currentBatch int
	batchSize    int
	description  string
	startTime    time.Time
}

func NewBatchProgressIndicator(totalBatches, batchSize int, description string) *BatchProgressIndicator {
	return &BatchProgressIndicator{
		totalBatches: totalBatches,
		currentBatch: 0,
		batchSize:    batchSize,
		description:  description,
		startTime:    time.Now(),
	}
}

func (b *BatchProgressIndicator) UpdateBatch(currentBatch int) {
	b.currentBatch = currentBatch
	elapsed := time.Since(b.startTime)

	var etaStr string
	if currentBatch > 0 {
		rate := float64(currentBatch) / elapsed.Seconds()
		if rate > 0 {
			remaining := float64(b.totalBatches-currentBatch) / rate
			eta := time.Duration(remaining) * time.Second
			etaStr = fmt.Sprintf(" ETA: %s", formatDuration(eta))
		}
	}

	fmt.Printf("\r%s: Batch %d/%d (%d items) %s%s",
		b.description, currentBatch, b.totalBatches, currentBatch*b.batchSize,
		formatDuration(elapsed), etaStr)
	_ = os.Stdout.Sync()
}

func (b *BatchProgressIndicator) Complete() {
	elapsed := time.Since(b.startTime)
	totalItems := b.totalBatches * b.batchSize
	fmt.Printf("\r%s: Completed %d batches (%d items) in %s\n",
		b.description, b.totalBatches, totalItems, formatDuration(elapsed))
}
