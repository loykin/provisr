package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/loykin/provisr"
)

// This example demonstrates starting and stopping a group of processes
// together using the public provisr facade.
func main() {
	mgr := provisr.New()
	g := provisr.NewGroup(mgr)

	gs := provisr.ServiceGroup{
		Name: "demo-group",
		Members: []provisr.Spec{
			{Name: "web", Command: "sh -c 'echo web start; sleep 2'"},
			{Name: "worker", Command: "sh -c 'echo worker start; sleep 2'", Instances: 2},
		},
	}

	if err := g.Start(gs); err != nil {
		panic(err)
	}

	stmap, err := g.Status(gs)
	if err != nil {
		panic(err)
	}
	b, _ := json.MarshalIndent(stmap, "", "  ")
	fmt.Println("Status after start:")
	fmt.Println(string(b))

	// Wait a bit and then stop all members
	time.Sleep(1500 * time.Millisecond)
	_ = g.Stop(gs, 2*time.Second)

	stmap2, _ := g.Status(gs)
	b2, _ := json.MarshalIndent(stmap2, "", "  ")
	fmt.Println("Status after stop:")
	fmt.Println(string(b2))
}
