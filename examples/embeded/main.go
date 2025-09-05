package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/loykin/provisr"
)

// A tiny embedded example using the public provisr facade directly,
// without importing internal packages.
func main() {
	mgr := provisr.New()
	// Optional: set some global env
	mgr.SetGlobalEnv([]string{"GREETING=hello"})

	spec := provisr.Spec{
		Name:          "embedded-demo",
		Command:       "sh -c 'echo $GREETING from embedded; sleep 1'",
		RetryCount:    0,
		RetryInterval: 0,
	}
	if err := mgr.Start(spec); err != nil {
		panic(err)
	}
	// Query status
	st, _ := mgr.Status("embedded-demo")
	b, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println(string(b))

	// Stop after a moment
	time.Sleep(1500 * time.Millisecond)
	_ = mgr.Stop("embedded-demo", 2*time.Second)
	st2, _ := mgr.Status("embedded-demo")
	b2, _ := json.MarshalIndent(st2, "", "  ")
	fmt.Println(string(b2))
}
