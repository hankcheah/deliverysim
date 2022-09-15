package main

import "fmt"

type RawOrder struct {
	Id       string `json:id`
	PrepTime int    `json:prepTime`
	Name     string `json:name`
}

type Order struct {
	RawOrder

	PlacedAt          int64 // The time when the order is placed by the customer
	KitchenReceivedAt int64 // The time when the kitchen receives the order
	ReadyAt           int64 // The time when the order is cooked and ready for courier pickup
	PickedUpAt        int64 // The time when the food is picked up by the courier

	CourierDispatchedAt int64 // The time when the courier is dispatched
	CourierArrivedAt    int64 // The time when the courier arrives and is ready to pick up the order
}

func (o Order) String() string {
	return fmt.Sprintf("Order ID: %v PrepTime: %v Name: %v PlacedAt: %v KitchenReceivedAt: %v ReadyAt: %v PickedUpAt: %v CourierDispatchedAt: %v CourierArrivedAt: %v",
		o.Id, o.PrepTime, o.Name, o.PlacedAt, o.KitchenReceivedAt, o.ReadyAt, o.PickedUpAt, o.CourierDispatchedAt, o.CourierArrivedAt)
}
