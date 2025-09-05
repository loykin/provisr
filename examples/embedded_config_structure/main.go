package main

import (
	"encoding/json"
	"fmt"

	"github.com/loykin/provisr"
)

// This example demonstrates constructing a process spec in code (struct-based) and launching it.
func main() {
	mgr := provisr.New()
	// Global env you might otherwise load from config
	mgr.SetGlobalEnv([]string{"HELLO=world"})
	// Build the spec directly
	sp := provisr.Spec{
		Name:    "struct-demo",
		Command: "sh -c 'echo $HELLO from struct; sleep 1'",
	}
	if err := mgr.Start(sp); err != nil {
		panic(err)
	}
	st, _ := mgr.Status("struct-demo")
	b, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println(string(b))
}
