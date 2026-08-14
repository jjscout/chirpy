package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

func handleHealthz(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Increment your counter here (runs on every request)
		cfg.fileserverHits.Add(1)
        
        // 2. Delegate the request to the inner handler
        next.ServeHTTP(w, r)
    })
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
    cfg.fileserverHits.Store(0)
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Hits reset to 0"))
}

func (cfg *apiConfig) handleRequestsCount(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(fmt.Sprintf(
		`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`,
		cfg.fileserverHits.Load(),
	)))
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}

func main() {

	apiCfg := apiConfig{}

	serveMux := http.NewServeMux()

	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}
	serveMux.HandleFunc("GET /api/healthz", handleHealthz)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.handleRequestsCount)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	serveMux.Handle(
		"/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))),
	)

	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}
