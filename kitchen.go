// This file contains types, methods, and functions related to kitchen.

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type Kitchen struct {
	Capacity int // Number of workers that can prepare orders simultaneously

	// Number of orders that can fit on the counter for courier pickup.
	// When the counter is full, the kitchen workers will have to stop cooking.
	CounterSize int

	workersReady chan interface{} // Used to indicate worker readiness

	orderIn       <-chan Order // Incoming orders
	readyOrdersCh chan Order   // Outgoing orders: i.e. cooked food ready for courier pickup

	expectedOrderCount  int
	processedOrderCount int
	mu                  sync.Mutex
}

type KitchenConfig struct {
	Capacity    int
	CounterSize int
	OrderIn     <-chan Order
}

// NewKitchen returns a new Kitchen-typed variable.
// It also creates the workers.
func NewKitchen(cfg KitchenConfig, expectedOrderCount int) *Kitchen {
	k := &Kitchen{
		Capacity:    cfg.Capacity,
		CounterSize: cfg.CounterSize,

		expectedOrderCount:  expectedOrderCount,
		processedOrderCount: 0,

		orderIn:       cfg.OrderIn,
		readyOrdersCh: make(chan Order, cfg.CounterSize),
		workersReady:  make(chan interface{}, cfg.Capacity),
	}

	k.spawnWorkers()

	return k
}

// spawnWorkers creates worker goroutines.
func (k *Kitchen) spawnWorkers() {
	for i := 0; i < k.Capacity; i++ {
		go k.worker(i, k.orderIn, k.readyOrdersCh, k.workersReady)
	}
}

// worker contains the logic and work flow for the kitchen workers.
func (k *Kitchen) worker(id int, orderIn <-chan Order, readyOrders chan<- Order, workersReady chan interface{}) {
	log.Debug(fmt.Sprintf("Kitchen worker %v is up.\n", id))
	defer log.Debug(fmt.Sprintf("Kitchen worker %v has retired.\n", id))

	// This worker is ready for work
	workersReady <- struct{}{}

	for {
		select {
		case order, more := <-orderIn:
			if more {
				order.KitchenReceivedAt = time.Now().Unix()
				fmt.Printf("[ORDER RECEIVED] Kitchen worker %v is working on order %v (prep time: %v s) (Time: %v)\n", id, order.Id, order.PrepTime, order.KitchenReceivedAt)

				time.Sleep(time.Duration(order.PrepTime) * time.Second)
				log.Debug(fmt.Sprintf("Kitchen worker %v has finished cooking order %v.\n", id, order.Id))

				order.ReadyAt = time.Now().Unix()
				fmt.Printf("[ORDER READY] Order %v is ready for pickup. (Time: %v)\n", order.Id, order.ReadyAt)

				readyOrders <- order

				// Acquire a lock so the kitchen stats can be updated
				k.mu.Lock()
				k.processedOrderCount++

				log.Debug(fmt.Sprintf("Kitchen work completion: %v %%\n", 100*k.processedOrderCount/k.expectedOrderCount))
				if k.processedOrderCount >= k.expectedOrderCount {
					log.Debug("Kitchen has processed all orders.")
					close(readyOrders)
					k.mu.Unlock()
					return
				}
				k.mu.Unlock()
			} else {
				log.Debug(fmt.Sprintf("Kitchen worker %v has no more work.\n", id))
				return
			}
		}
	}
}

// Ready returns a channel that indicates worker readiness.
func (k *Kitchen) Ready() <-chan interface{} {
	ready := make(chan interface{})

	go func() {
		workerCount := 0
		for {
			select {
			case <-k.workersReady:
				workerCount++
				if workerCount == k.Capacity {
					log.Debug(fmt.Sprintf("All %v kitchen workers are ready.\n", workerCount))

					// This sends a non-blocking broadcast on the channel
					close(ready)
					return
				}
			case <-time.After(10 * time.Second):
				panic("Error. Kitchen setup timeout.")
				return
			}
		}
	}()

	return ready
}

// ReadyOrders returns a channel that will be sent Order's that are ready for courier pickup.
func (k *Kitchen) ReadyOrders() <-chan Order {
	if k.readyOrdersCh == nil {
		panic("Kitchen output channel hasn't been set up properly.")
	}
	return k.readyOrdersCh
}
