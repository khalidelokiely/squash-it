package router

import (
	"errors"
	"fmt"
	"log"
	"net/http"
)

// Spin our http server
func (r *Router) Spin() *http.Server {
	server := &http.Server{
		Addr:         ":" + r.opts.Port,
		Handler:      r.mux,
		ReadTimeout:  r.opts.ReadTimeout,
		WriteTimeout: r.opts.WriteTimeout,
	}

	go func() {
		fmt.Println("net/http Server Listening on :" + r.opts.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	return server
}
