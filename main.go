package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
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
	dbQueries      *database.Queries
	platform       string
	jwtSecret      string
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
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	err := cfg.dbQueries.DeleteUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("error: %v", err))
		return
	}

	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

func (cfg *apiConfig) handleRequestsCount(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(w,
		`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`,
		cfg.fileserverHits.Load())
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

var profanities = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
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

func validate(chirp string) bool {
	return len(chirp) <= 140
}

func (cfg *apiConfig) handleCreateChirp(w http.ResponseWriter, request *http.Request) {
	type parameters struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}
	type response struct {
		Id        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserId    uuid.UUID `json:"user_id"`
	}
	token, err := auth.GetBearerToken(request.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	decoder := json.NewDecoder(request.Body)
	p := parameters{}
	err = decoder.Decode(&p)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	if !validate(p.Body) {
		respondWithError(w, 400, "Chirp too long")
		return
	}

	filtered := replaceProfanities(p.Body, profanities)
	chirp, err := cfg.dbQueries.CreateChirp(request.Context(), database.CreateChirpParams{
		Body:   filtered,
		UserID: userID,
	})
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	respondWithJSON(w, 201, response{
		Id:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
	})
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

	filtered := replaceProfanities(p.Body, profanities)
	respondWithJSON(w, 200, response{Body: filtered})
}

type createResponse struct {
	Id           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

func (cfg *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	type createRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	p := createRequest{}
	err := decoder.Decode(&p)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	passwordHash, err := auth.HashPassword(p.Password)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	createUserParams := database.CreateUserParams{
		Email:          p.Email,
		HashedPassword: passwordHash,
	}
	user, err := cfg.dbQueries.CreateUser(r.Context(), createUserParams)

	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}

	respondWithJSON(
		w,
		201,
		createResponse{
			Id:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		},
	)
}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type loginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	p := loginRequest{}
	err := decoder.Decode(&p)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	user, err := cfg.dbQueries.GetUserByEmail(r.Context(), p.Email)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	passwordMatch, err := auth.CheckPasswordHash(p.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	if !passwordMatch {
		respondWithError(w, 401, "Invalid credentials")
		return
	}
	accessTokenDuration := time.Hour
	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, accessTokenDuration)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	refreshToken := auth.MakeRefreshToken()

	refreshTokenDuration := 60 * 24 * time.Hour
	_, err = cfg.dbQueries.AddRefreshToken(r.Context(), database.AddRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresIn: int32(refreshTokenDuration.Hours()),
	})
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	respondWithJSON(w, 200, createResponse{
		Id:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
	})

}

func (cfg *apiConfig) handleRefresh(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	refreshToken, err := cfg.dbQueries.GetValidRefreshToken(r.Context(), token)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	accessTokenDuration := time.Hour
	accessToken, err := auth.MakeJWT(refreshToken.UserID, cfg.jwtSecret, accessTokenDuration)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}

	respondWithJSON(w, 200, createResponse{
		Token: accessToken,
	})

}

func (cfg *apiConfig) handleRevoke(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	_, err = cfg.dbQueries.RevokeRefreshToken(r.Context(), token)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	respondWithJSON(w, 204, nil)

}

func (cfg *apiConfig) handleGetChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	type chirpResponse struct {
		Id        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserId    uuid.UUID `json:"user_id"`
	}

	chirpResponses := make([]chirpResponse, len(chirps))
	for i, chirp := range chirps {
		chirpResponses[i] = chirpResponse{
			Id:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserId:    chirp.UserID,
		}
	}

	respondWithJSON(w, 200, chirpResponses)
}

func (cfg *apiConfig) handleGetChirpByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idU, err := uuid.Parse(id)
	fmt.Printf("idU: %v, err: %v\n", idU, err)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("error: %v", err))
		return
	}
	chirp, err := cfg.dbQueries.GetChirpByID(r.Context(), idU)
	fmt.Printf("chirp: %v, err: %v\n", chirp, err)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, 404, "Chirp not found")
			return
		}
		respondWithError(w, 500, fmt.Sprintf("database query error: %v", err))
		return
	}
	type chirpResponse struct {
		Id        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserId    uuid.UUID `json:"user_id"`
	}

	response := chirpResponse{
		Id:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
	}

	respondWithJSON(w, 200, response)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("warning: couldn't load .env file: %v\n", err)
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("DB_URL must be set")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	if err = db.Ping(); err != nil {
		fmt.Printf("error connecting to database: %v\n", err)
		os.Exit(1)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		fmt.Println("JWT_SECRET must be set")
		os.Exit(1)
	}

	dbQueries := database.New(db)
	apiCfg := apiConfig{
		dbQueries: dbQueries,
		platform:  os.Getenv("PLATFORM"),
		jwtSecret: jwtSecret,
	}

	serveMux := http.NewServeMux()

	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}
	serveMux.HandleFunc("GET /api/healthz", handleHealthz)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.handleRequestsCount)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	serveMux.HandleFunc("POST /api/validate_chirp", apiCfg.handleValidateChirp)
	serveMux.HandleFunc("POST /api/chirps", apiCfg.handleCreateChirp)
	serveMux.HandleFunc("POST /api/users", apiCfg.handleCreateUser)
	serveMux.HandleFunc("POST /api/login", apiCfg.handleLogin)
	serveMux.HandleFunc("POST /api/refresh", apiCfg.handleRefresh)
	serveMux.HandleFunc("POST /api/revoke", apiCfg.handleRevoke)
	serveMux.HandleFunc("GET /api/chirps", apiCfg.handleGetChirps)
	serveMux.HandleFunc("GET /api/chirps/{id}", apiCfg.handleGetChirpByID)
	serveMux.Handle(
		"/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))),
	)

	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}
