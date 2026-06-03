package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	mgr := provisr.New()
	base := os.Getenv("API_BASE") // e.g. "/abc"
	if base == "" {
		base = "/api"
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Mount provisr API under base path
	apiRouter := provisr.NewRouter(mgr, base)
	r.Any(base+"/*any", gin.WrapH(apiRouter.Handler()))
	// Also support exact paths to avoid 404 on no extra segment
	r.Any(base, gin.WrapH(apiRouter.Handler()))

	// Start a demo process so you can see it in /status (2 instances)
	_ = mgr.RegisterN(provisr.Spec{
		Name:      "demo",
		Command:   "/bin/sh -c 'while true; do echo demo; sleep 5; done'",
		Instances: 2,
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Println("starting gin server on :8080 with base", base)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
