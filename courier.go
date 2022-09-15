// This file contains type definition, methods, and functions related to the couriers.

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Courier dispatch strategies
type CourierStrategy int

const (
	CourierStrategyFIFO CourierStrategy = iota
	CourierStrategyMatched
)

// CourierPool manages individual couriers.
type CourierPool struct {
	Capacity   int             // How many couriers
	Strategy   CourierStrategy // Dispatch strategy
	TravelTime [2]int          // Range of delay between courier dispatch and arrival

	dispatchOrders <-chan Order // Pool-level dispatch channel
	pickupOrders   <-chan Order // Pool-level kitchen channel

	workerDispatchOrders chan CourierOrder // Pool-to-workers dispatch orders.
	workerPickupOrders   chan Order        // Pool-to-workers kitchen orders (i.e. orders that are ready for courier pickup).

	workersReady    chan interface{} // Semaphore used to show worker readiness.
	deliveredOrders chan Order       // Used to communicate orders that have been delivered

	expectedOrderCount  int
	processedOrderCount int

	analytics *Analytics
}

// CourierOrder is a lightweight type used in communicate between
// the pool and the worker.
// Author: I don't think this is a great name but this will do for now.
type CourierOrder struct {
	Order   Order
	Channel chan Order // Direct channel to the courier
}

// NewCourierPool constructs a new CourierPool and spawns workers.
func NewCourierPool(capacity int,
	strategy string,
	travelTime [2]int,
	expectedOrderCount int,
	analytics *Analytics,
	dispatchOrders <-chan Order,
	pickupOrders <-chan Order,
	orderRate int) *CourierPool {
	p := &CourierPool{
		Capacity:   capacity,
		TravelTime: travelTime,

		expectedOrderCount:  expectedOrderCount,
		processedOrderCount: 0,

		// Pool-level channels
		dispatchOrders: dispatchOrders,
		pickupOrders:   pickupOrders,

		// Worker-level channels
		deliveredOrders: make(chan Order),

		workersReady: make(chan interface{}, capacity),
		analytics:    analytics,
	}

	rand.Seed(time.Now().UnixNano())
	p.Strategy = p.convStrategy(strategy)

	// The ideal channel size is a function of the rate of Orders Per Second,
	// Courier Pool Size, and Average Travel Time
	// orderRate * travelTime[1] - theoretical upperbound for number of dispatch orders in the channel
	// multiply by 2 - make sure there's enough space for the service worker to switch between
	// dispatch and pickup
	channelSize := 2 * orderRate * travelTime[1]

	if channelSize < 2*capacity {
		channelSize = 2 * capacity
	}

	p.workerPickupOrders = make(chan Order, channelSize)
	p.workerDispatchOrders = make(chan CourierOrder, channelSize)

	// Spawn worker goroutines
	p.spawnServiceWorkers(p.dispatchOrders, p.pickupOrders)
	p.spawnCourierWorkers()

	return p
}

// convStrategy converts string to CourierStrategy type
func (p *CourierPool) convStrategy(stratStr string) CourierStrategy {
	switch strings.ToLower(stratStr) {
	case "fifo":
		return CourierStrategyFIFO
	case "matched":
		return CourierStrategyMatched
	default:
		panic(fmt.Sprintf("Invalid dispatch strategy: %v\n", stratStr))
	}
}

// spawnServiceWorkers creates service workers that manage the pool and the couriers.
func (p *CourierPool) spawnServiceWorkers(poolDispatchOrders <-chan Order, poolPickupOrders <-chan Order) {
	directChannels := sync.Map{}

	for i := 0; i < 5; i++ {
		go p.serviceWorker(poolDispatchOrders, poolPickupOrders, &directChannels)
	}
}

// serviceWorker creates a service worker that manages the pool and courier workers
func (p *CourierPool) serviceWorker(poolDispatchOrders <-chan Order, poolPickupOrders <-chan Order, directChannels *sync.Map) {
	for {
		select {
		case order, more := <-poolDispatchOrders:
			if more {
				log.Debug(fmt.Sprintf("Courier pool receives dispatch order for order: %v\n", order.Id))

				if p.Strategy == CourierStrategyMatched {
					// This creates a direct channel between the kitchen order and the matched courier
					channel := make(chan Order)
					p.workerDispatchOrders <- CourierOrder{order, channel}
					directChannels.Store(order.Id, channel)
				} else if p.Strategy == CourierStrategyFIFO {
					// For FIFO, no matchmaking is needed between kitchen orders and the couriers.
					// The couriers will listen to the same general kitchen channel.
					// More details in the kitchen order handling code below.
					p.workerDispatchOrders <- CourierOrder{order, nil}
				} else {
					panic(fmt.Sprintf("%v is not a valid pooling strategy.\n", p.Strategy))
				}
			}
		case order, more := <-poolPickupOrders:
			if more {
				log.Debug(fmt.Sprintf("Courier pool receives an order from kitchen. Order id: %v\n", order.Id))

				// Pass kitchen order to the workers
				if p.Strategy == CourierStrategyMatched {
					// Use the direct channel established earlier
					// This channel allows direct, one-time communication between the order and the matched courier.
					value, ok := directChannels.Load(order.Id)
					if ok {
						if directCh, ok := value.(chan Order); ok {
							directCh <- order
						} else {
							panic(fmt.Sprintf("Direct channel for order %v is not even a channel?\n", order.Id))
						}
					} else {
						panic(fmt.Sprintf("Direct channel for order %v is missing. Simulation correctness is no longer guaranteed.\n", order.Id))
					}
				} else if p.Strategy == CourierStrategyFIFO {
					// For FIFO, just send the kitchen order to the general channel
					// the first courier to arrive gets to deliver the order.
					p.workerPickupOrders <- order
				} else {
					panic(fmt.Sprintf("%v is not a valid pooling strategy.\n", p.Strategy))
				}
			} else {
				log.Debug("Courier pool has received all the orders from kitchen.\n")
				return
			}
		}
	}
}

// spawnWorkers crreates worker goroutines.
func (p *CourierPool) spawnCourierWorkers() {
	for i := 0; i < p.Capacity; i++ {
		go p.worker(i, p.workerDispatchOrders, p.workerPickupOrders, p.deliveredOrders, p.workersReady)
	}
}

// worker contains the logic and work flow for the courier.
func (p *CourierPool) worker(id int,
	dispatchOrders <-chan CourierOrder,
	pickupOrders <-chan Order,
	deliveredOrders chan Order,
	workersReady chan interface{},
) {

	log.Debug(fmt.Sprintf("Courier %v is up.\n", id))
	defer log.Debug(fmt.Sprintf("Courier %v has retired.\n", id))

	// This worker is ready for work
	workersReady <- struct{}{}

	for {
		dispatchInfo, more := <-dispatchOrders
		if more {
			// Randomize courier travel time
			travelTime := p.genTravelTime()

			dispatchedAt := time.Now().Unix()
			fmt.Printf("[COURIER DISPATCHED] Courier %v has been dispatched. ETA: %v s. (Time: %v)\n", id, travelTime, dispatchedAt)
			time.Sleep(time.Duration(travelTime) * time.Second)

			arrivedAt := time.Now().Unix()
			fmt.Printf("[COURIER ARRIVED] Courier %v has arrived. (Time: %v)\n", id, arrivedAt)

			var order Order
			if dispatchInfo.Channel != nil {
				log.Debug(fmt.Sprintf("Courier %v is waiting to pick up order %v.\n", id, dispatchInfo.Order.Id))
				// Read order from the direct channel. Used in "Matched" strategy.
				order = <-dispatchInfo.Channel
			} else {
				log.Debug(fmt.Sprintf("Courier %v is waiting to pick up any order.\n", id))
				// Read order from the pool
				order = <-pickupOrders
			}

			order.PickedUpAt = time.Now().Unix()
			order.CourierDispatchedAt = dispatchedAt
			order.CourierArrivedAt = arrivedAt

			fmt.Printf("[ORDER PICKED UP] Order %v has been picked up by Courier %v. (Time: %v)\n", order.Id, id, order.PickedUpAt)
			fmt.Printf("[ORDER DELIVERED] Order %v has been delivered. (Time: %v)\n", order.Id, order.PickedUpAt)

			// Update and display metrics
			// These methods are atomic
			p.analytics.UpdateMetrics(order)
			p.analytics.PrintMetrics()

			deliveredOrders <- order
		} else {
			log.Debug(fmt.Sprintf("Courier %v is not getting dispatched.\n", id))
			return
		}
	}
}

// genTravelTime randomizes travel time within the range specified by the user.
func (p *CourierPool) genTravelTime() int {
	if p.TravelTime[0] == 0 && p.TravelTime[1] == 0 {
		return 0
	}
	return rand.Intn(p.TravelTime[1]-p.TravelTime[0]) + p.TravelTime[0]
}

// Ready returns a channel that indicates all workers are up and ready for tasks.
// To use this method to block until all workers are live, call
// <-p.Ready()
func (p *CourierPool) Ready() <-chan interface{} {
	ready := make(chan interface{})

	go func() {
		workerCount := 0
		for {
			select {
			case <-p.workersReady:
				workerCount++
				if workerCount == p.Capacity {
					log.Debug(fmt.Sprintf("All %v couriers are ready.\n", workerCount))

					// This sends a non-blocking broadcast on the channel
					close(ready)
					return
				}
			case <-time.After(10 * time.Second):
				panic("Error. Courier pool setup timeout.")
				return
			}
		}
	}()

	return ready
}

// DeliveredOrders return a read-only channel that will be sent orders that have been delivered.
func (p *CourierPool) DeliveredOrders() <-chan Order {
	if p.deliveredOrders == nil {
		panic("CourierPool's ordersDelivered channel hasn't been set up properly.")
	}
	return p.deliveredOrders
}
