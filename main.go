package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Invalid command usage")
		fmt.Println("[Usage]")
		fmt.Println("./deliverysim <Profile Definition File>")
		fmt.Println("[Example]")
		fmt.Println("./deliverysim profile/default.json")
		panic("Exiting...")
	}

	profileFile := os.Args[1]

	sim := NewSimulation(profileFile)
	if sim.DebugMode() {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	sim.Start()
}
