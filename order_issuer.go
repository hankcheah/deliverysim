package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
	"time"
)

type orderIssuer struct {
	Rate   int     // Number of orders per second
	Count  int     // Max number of orders to issue
	orders []Order // Orders

	listeners []chan Order // channels to broadcast orders to
	mu        sync.RWMutex
}

// NewOrderIssuer returns a new orderIssuer.
func NewOrderIssuer(sourceFile string, rate int, specifiedLimit int) *orderIssuer {
	oi := &orderIssuer{
		Rate:      rate,
		listeners: []chan Order{},
	}

	oi.orders = oi.extractOrders(sourceFile)

	// Determine the number of orders to dish out
	// TODO: Go beyond the number of orders in the source file
	oi.Count = len(oi.orders)
	if specifiedLimit > 0 && specifiedLimit < oi.Count {
		oi.Count = specifiedLimit
	}

	return oi
}

// Orders return a read channel that will be sent orders.
func (oi *orderIssuer) Orders() <-chan Order {
	listener := make(chan Order, oi.Count)

	oi.mu.Lock()
	oi.listeners = append(oi.listeners, listener)
	oi.mu.Unlock()

	return listener
}

// broadcast sends data on all listening channels.
func (oi *orderIssuer) broadcast(o Order) {
	oi.mu.Lock()
	for i, l := range oi.listeners {
		log.Debug(fmt.Sprintf("Order issuer sending order %v listener %v\n", o.Id, i))
		l <- o
	}
	oi.mu.Unlock()
}

// stopBroadcast closes the listening channels.
func (oi *orderIssuer) stopBroadcast() {
	oi.mu.Lock()
	for i, l := range oi.listeners {
		log.Debug(fmt.Sprintf("Order issuer closing listener %v\n", i))
		close(l)
	}
	oi.mu.Unlock()
}

// Start creates a goroutine that issues orders to listening channels.
func (oi *orderIssuer) Start() {
	log.Debug("Order issuer will now start issuing orders.\n")

	go func() {
		count := 1
		tStart := time.Now()
		for i := 0; i < oi.Count; i++ {
			order := oi.orders[i]
			log.Debug(fmt.Sprintf("Order issuer sends order %v (Completion: %v %%)\n", order.Id, 100*(i+1)/oi.Count))

			order.PlacedAt = time.Now().Unix()
			oi.broadcast(order)
			count++

			// Rate-limiting. It's a little primitive but it should work for most practical order rate
			if count > oi.Rate {
				time.Sleep(1000*time.Millisecond - time.Since(tStart))
				count = 1
				tStart = time.Now()
			}
		}

		// Issue broadcast on channel: all orders have been issued.
		log.Debug("Order issuer: all orders have been sent")
		oi.stopBroadcast()
	}()
}

// extractOrders extracts orders from the user-specified JSON file and convert it to a slice of Order
func (oi *orderIssuer) extractOrders(sourceFile string) []Order {
	orders := []Order{}

	data, err := os.ReadFile(sourceFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to open orders data json file.\n%v", err))
	}

	err = json.Unmarshal(data, &orders)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode JSON order data.\n%v", err))
	}

	return orders
}
