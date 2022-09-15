// This file contains the type definition and methods for Analytics.
// Analytics is a simplistic stats-keeping type that keeps track of
// metrics like average food wait time and courier wait time.

package main

import (
	"fmt"
	"sync"
)

type Analytics struct {
	totalFoodWaitTime    int64 // All time variables here are measured in seconds
	totalCourierWaitTime int64
	count                int64 // Number of data points

	avgFoodWaitTime    float64
	avgCourierWaitTime float64
	mu                 sync.RWMutex

	DebugMode bool
}

// UpdateMetrics refreshes the metrics given a new data point.
func (a *Analytics) UpdateMetrics(o Order) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update count and total
	a.count++
	a.totalFoodWaitTime += o.PickedUpAt - o.ReadyAt
	a.totalCourierWaitTime += o.PickedUpAt - o.CourierArrivedAt

	// Refreshes the averages
	a.avgFoodWaitTime = float64(a.totalFoodWaitTime) / float64(a.count)
	a.avgCourierWaitTime = float64(a.totalCourierWaitTime) / float64(a.count)
}

// PrintMetrics prints the current metrics
func (a *Analytics) PrintMetrics() {
	fmt.Println(a.GetSummary())
}

// GetSummary returns a string containing the current metrics
func (a *Analytics) GetSummary() string {
	// Acquire a read lock
	a.mu.RLock()
	defer a.mu.RUnlock()

	res := fmt.Sprintf("Avg. food wait time (s): %.2f\nAvg. courier wait time (s): %.2f\n", a.avgFoodWaitTime, a.avgCourierWaitTime)

	if a.DebugMode {
		res = fmt.Sprintf("%vTotal food wait time (s): %v\nTotal courier wait time (s): %v\nCount: %v\n", res, a.totalFoodWaitTime, a.totalCourierWaitTime, a.count)
	}
	return res
}
