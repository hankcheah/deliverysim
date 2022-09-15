package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type simulation struct {
	Profile SimulationProfile
}

type SimulationProfile struct {
	Name  string `json:Name`
	Desc  string `json:Desc`
	Order struct {
		SourceFile string `json:SoureFile`
		Rate       int    `json:Rate`
		Limit      int    `json:Limit`
	}
	Kitchen struct {
		Capacity    int `json:Capacity`
		CounterSize int `json:CounterSize`
	}
	Courier struct {
		DispatchStrategy string `json:DispatchStrategy`
		PoolSize         int    `json:PoolSize`
		TravelTime       [2]int `json:TravelTime`
	}
	DebugMode bool `json:DebugMode`
}

// NewSimulation returns a new simulation instance.
func NewSimulation(profileFile string) *simulation {
	s := &simulation{}

	var profile SimulationProfile

	data, err := os.ReadFile(profileFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to open profile definition file:\n%v", err))
	}

	err = json.Unmarshal(data, &profile)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode profile json data.\n%v", err))
	}

	s.Profile = profile

	return s
}

// DebugMode indicates if the simulation is running in debug mode.
func (s *simulation) DebugMode() bool {
	return s.Profile.DebugMode
}

// Describe prints the details of the simulation profile.
func (s *simulation) Describe() {
	fmt.Printf("Profile name: %v\n", s.Profile.Name)
	fmt.Printf("Description: %v\n", s.Profile.Desc)
	fmt.Printf("Order source: %v\n", s.Profile.Order.SourceFile)
	fmt.Printf("Order rate (per second): %v\n", s.Profile.Order.Rate)

	orderLimit := fmt.Sprintf("Order limit: %v", s.Profile.Order.Limit)
	if s.Profile.Order.Limit == -1 {
		orderLimit += " (all available orders)"
	}
	orderLimit += "\n"
	fmt.Printf(orderLimit)

	fmt.Printf("Kitchen capacity: %v\n", s.Profile.Kitchen.Capacity)
	fmt.Printf("Kitchen counter size: %v\n", s.Profile.Kitchen.CounterSize)
	fmt.Printf("Courier pool size: %v\n", s.Profile.Courier.PoolSize)
	fmt.Printf("Courier dispatch strategy: %v\n", s.Profile.Courier.DispatchStrategy)
	fmt.Printf("Courier travel time: Between %v to %v seconds (inclusive).\n", s.Profile.Courier.TravelTime[0], s.Profile.Courier.TravelTime[1])
	fmt.Printf("\nDebug Mode: %v\n\n", s.Profile.DebugMode)
}

// Start sets up and manages the simulation lifecycle.
func (s *simulation) Start() {
	s.Describe()

	oi := NewOrderIssuer(
		s.Profile.Order.SourceFile, // Order source file
		s.Profile.Order.Rate,       // Order rate (orders per second)
		s.Profile.Order.Limit,      // Max number of orders to issue
	)

	k := NewKitchen(
		KitchenConfig{
			Capacity:    s.Profile.Kitchen.Capacity,    // How many cooks in the kitchen
			CounterSize: s.Profile.Kitchen.CounterSize, // How many orders ready for courier pickup can the restaurant counter accomodate?
			OrderIn:     oi.Orders(),                   // Order source
		},
		oi.Count, // Tell the kitchen how many orders to expect. Necessary for a simulation but not necessary in the real world.
	)

	analytics := Analytics{DebugMode: s.Profile.DebugMode}
	cPool := NewCourierPool(
		s.Profile.Courier.PoolSize, // Number of couriers the fulfilment company employs
		s.Profile.Courier.DispatchStrategy,
		s.Profile.Courier.TravelTime,
		oi.Count,
		&analytics,
		oi.Orders(),
		k.ReadyOrders(),
		oi.Rate,
	)

	// Sync-up: make sure all workers are ready to go
	<-k.Ready()     // All kitchen workers are live and ready
	<-cPool.Ready() // All couriers are live and ready

	fmt.Println("========== Simulation has started ==========")
	tStart := time.Now()

	// Start issuing orders
	oi.Start()

	// A blocking call that monitors the progress of the simulation
	s.monitor(cPool.DeliveredOrders(), oi.Count, tStart)

	fmt.Println("========== Simulation has completed ==========")
	fmt.Printf("Time spent: %.1f s.\n\n", time.Since(tStart).Seconds())
	fmt.Println("Simulation results:")
	fmt.Println(analytics.GetSummary())
}

// monitor keeps a close eye on the simulation progress.
// It also blocks until the simulation is complete (when all orders have been delivered).
func (s *simulation) monitor(counter <-chan Order, expectedCount int, tStart time.Time) {
	count := 0
	for {
		_, more := <-counter
		if more {
			count++
			fmt.Printf("Simulation progress: %v %% (%v out of %v) Time lapsed: %.1f s.\n", 100*count/expectedCount, count, expectedCount, time.Since(tStart).Seconds())
			if count == expectedCount {
				return
			}
		} else {
			return
		}
	}
}
