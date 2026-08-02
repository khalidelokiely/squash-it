package router

import "net/http"

// Spin our http server
func (r *Router) Spin() {
	server := &http.Server{
		Addr:         ":" + r.opts.Port,
		Handler:      r.mux,
		ReadTimeout:  r.opts.ReadTimeout,
		WriteTimeout: r.opts.WriteTimeout,
	}

	err := server.ListenAndServe()

	if err != nil {
		panic(err)
	}
}
