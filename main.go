package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"encoding/json"
	"strings"
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

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type Errval struct {
		Errstr string `json:"error"`
	}
	respondWithJSON(w, code, Errval{Errstr: msg})

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
			w.WriteHeader(500)
			return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func replaceProfanities(message string, blacklist []string) string {

	words := strings.Split(message, " ")
	blackSet := make(map[string]struct{}, len(blacklist))
	for _, word := range blacklist {
		blackSet[strings.ToLower(word)] = struct{}{}
	}
	var filtered []string
	for _, orw := range words {
		lw := strings.ToLower(orw)
		if _, found := blackSet[lw]; found {
			filtered = append(filtered, "****")
		} else {
			filtered = append(filtered, orw)
		}
	}
	return strings.Join(filtered, " ")
}

func (cfg *apiConfig) handleValidateChirp(w http.ResponseWriter, request *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type response struct {
		Body string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(request.Body)
	p := parameters{}
	err := decoder.Decode(&p)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	if len(p.Body) > 140 {
		respondWithError(w, 400, "Chirp too long")
		return
	}

	filtered := replaceProfanities(
		p.Body,
		[]string{
			"kerfuffle",
			"sharbert",
			"fornax",
		},
	)
	respondWithJSON(w, 200, response{Body: filtered})
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
	serveMux.HandleFunc("POST /api/validate_chirp", apiCfg.handleValidateChirp)
	serveMux.Handle(
		"/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))),
	)

	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}
