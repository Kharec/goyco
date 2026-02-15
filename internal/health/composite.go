package health

import (
	"context"
	"sync"
	"time"
)

type CompositeChecker struct {
	checkers []Checker
}

func NewCompositeChecker(checkers ...Checker) *CompositeChecker {
	return &CompositeChecker{
		checkers: checkers,
	}
}

func (c *CompositeChecker) AddChecker(checker Checker) {
	c.checkers = append(c.checkers, checker)
}

func (c *CompositeChecker) Check(ctx context.Context) OverallResult {
	results := make(map[string]Result)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, checker := range c.checkers {
		wg.Add(1)
		go func(ch Checker) {
			defer wg.Done()

			result := ch.Check(ctx)

			mu.Lock()
			results[ch.Name()] = result
			mu.Unlock()
		}(checker)
	}

	wg.Wait()

	return OverallResult{
		Status:    determineOverallStatus(results),
		Timestamp: time.Now().UTC(),
		Services:  results,
	}
}

func (c *CompositeChecker) CheckWithVersion(ctx context.Context, version string) OverallResult {
	result := c.Check(ctx)
	result.Version = version
	return result
}
