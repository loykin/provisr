package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	mng "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/server"
)

func main() {
	e := echo.New()
	mgr := mng.NewManager()
	base := os.Getenv("API_BASE")
	if base == "" {
		base = "/api"
	}

	r := server.NewRouter(mgr, base)
	h := r.Handler()

	// Mount under base using Echo's WrapHandler
	e.Any(base, echo.WrapHandler(h))
	e.Any(base+"/*", echo.WrapHandler(h))

	// Start a demo process so you can see it in /status (2 instances)
	_ = mgr.RegisterN(process.Spec{
		Name:      "demo",
		Command:   "/bin/sh -c 'while true; do echo demo; sleep 5; done'",
		Instances: 2,
	})

	log.Println("starting echo server on :8080 with base", base)
	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
