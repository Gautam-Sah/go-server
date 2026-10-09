package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"

	"database/sql"

	"random/internal/database"

	_ "github.com/lib/pq"

	"github.com/google/uuid"

	"time"

	"random/internal/auth"

	"sort"
)

type apiConfig struct{
	fileServerHits atomic.Int32
	dbQueries     *database.Queries
	JWTSecret string
	PolkaKey string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	IsChirpyRed bool    `json:"is_chirpy_red"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetrics(next http.Handler) http.Handler{
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){

		log.Printf("Path: %s", r.URL.Path)
		cfg.fileServerHits.Add(1)

		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileServerHits.Load()))) 

}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request){
		
		godotenv.Load()

		platform := os.Getenv("platform")

		if platform != "dev"{
			respondWithError(w, 403, "forbidden")
			return
		}

		cfg.fileServerHits.Store(0)
		err := cfg.dbQueries.DeleteAllUsers(r.Context())

		if err != nil{
			respondWithError(w, 500, "internal server error")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileServerHits.Load()))) 
		

}

func validationHandler(w http.ResponseWriter, r *http.Request){
	type jsonBody struct{
		Body string `json:"body"`
	}

	type ErrorResponse struct{
		Error string `json:"error"`
	}

	type success struct{
		Valid string `json:"valid"`
	}

	res := jsonBody{}

	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&res)

	if err != nil{
		log.Println(err)
		respondWithJSON(w, 500, ErrorResponse{"something went wrong"})
		return
	}

	if len(res.Body) > 140{
		fmt.Println(res.Body)
		respondWithJSON(w, 400, ErrorResponse{"too long"})
		return
	}


	words := strings.Split(strings.ToLower(res.Body), " ")

	clean := ""
	for _, word := range words{

		if word == "kerfuffle" || word == "sharbert" || word == "fornax" {
			clean += "**** " 
		} else{
			clean += word + " "
		}
		
	}

	respondWithJSON(w, 200, success{clean})

}

func respondWithError(w http.ResponseWriter, code int, msg string){
	    w.WriteHeader(code)
    	w.Write([]byte(msg))

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}){

	data, err := json.Marshal(payload)

	if err != nil{
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}
func main() {

	godotenv.Load()

	dbURL := os.Getenv("DB_URL")

	JWTSecret := os.Getenv("JWT_SECRET")

	PolkaKey := os.Getenv("POLKA_KEY")

	db, err := sql.Open("postgres", dbURL)

	if err != nil {
    log.Fatal(err)
}
	const port = "8080"

	mux := http.NewServeMux()

	apiCfg := apiConfig{
	dbQueries: database.New(db),
	JWTSecret: JWTSecret,
	PolkaKey: PolkaKey,
	}

	handler := http.StripPrefix("/app/", http.FileServer(http.Dir(".")))

	mux.Handle("/app/", apiCfg.middlewareMetrics(handler) )

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)

	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	mux.HandleFunc("POST /api/chirps", apiCfg.createChirp)

	mux.HandleFunc("GET /api/chirps", apiCfg.GetAllChirps)

	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.GetChirp)

	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirp)

	mux.HandleFunc("POST /api/users", apiCfg.createUser)

	mux.HandleFunc("PUT /api/users", apiCfg.updateUser)

	mux.HandleFunc("POST /api/login", apiCfg.getUser)

	mux.HandleFunc("POST /api/refresh", apiCfg.getNewToken)

	mux.HandleFunc("POST /api/revoke", apiCfg.revokeRefreshToken)

	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.upgradeToChirpyRed)
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(srv.ListenAndServe())
}


func (cfg *apiConfig) upgradeToChirpyRed(w http.ResponseWriter, r *http.Request){

	type User struct{ UserId uuid.UUID `json:"user_id"`} 


	type requestBody struct{
		Event string `json:"event"`
		Data  User `json:"data"`	

	}

	apiKey, err  := auth.GetAPIKey(r.Header)

	if err != nil{
		respondWithError(w, 401, err.Error())
		return
	}

	if apiKey != cfg.PolkaKey{
		respondWithError(w, 401, "invalid api key")
		return
	}


	req := requestBody{}

	err = json.NewDecoder(r.Body).Decode(&req)

	if err != nil{
		respondWithError(w, 500, " decode internal server error")
		return
	}

	if req.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}

	_, err = cfg.dbQueries.UpgradeUserToChirpyRed(r.Context(), req.Data.UserId)

	if err != nil {

		if err == sql.ErrNoRows{
			respondWithError(w, 404, "user no found")
			return
		}
		
		respondWithError(w, 500, err.Error())
		return
	}

	w.WriteHeader(204)

}

func (cfg *apiConfig) updateUser(w http.ResponseWriter, r *http.Request){

	token, err := auth.GetBearerToken(r.Header)

	if err != nil{
		respondWithError(w, 401, "missing token")
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.JWTSecret)

	if err != nil{
		respondWithError(w, 401, "unauthorized user")
		return
	}

	
	type requestBody struct{
		Email string `json:"email"`
		Password string `json:"password"`
	}

	req := requestBody{}

	decoder := json.NewDecoder(r.Body)

	err = decoder.Decode(&req)

	if err != nil{
		respondWithError(w, 500, "internal server error")
		return
	}

	hash, err := auth.HashPassword(req.Password)

		if err != nil{
		respondWithError(w, 500, "internal server error")
		return
	}

	user, err := cfg.dbQueries.UpdateUserCredentials(r.Context(), database.UpdateUserCredentialsParams{
			Email: req.Email,
			HashedPassword: hash,
			ID: userID,
			})

	if err != nil{
		respondWithError(w, 500, "internal server error")
		return
	}

	response := User{
		ID: user.ID,
		Email: user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		IsChirpyRed: user.IsChirpyRed,
	}

	respondWithJSON(w, 200, response)
}


func (cfg *apiConfig) revokeRefreshToken(w http.ResponseWriter, r *http.Request){

	token, err := auth.GetBearerToken(r.Header)

	if err != nil{
		respondWithError(w, 401, err.Error())
		return 
	}

	err = cfg.dbQueries.RevokeRefreshToken(r.Context(), token)

	if err != nil{
		 respondWithError(w, 500, "couldn't revoke token")
		return 
	}

	w.WriteHeader(204)

}

func (cfg *apiConfig) getNewToken(w http.ResponseWriter, r *http.Request){
 	token, err := auth.GetBearerToken(r.Header)

	if err != nil{
		respondWithError(w, 401, err.Error())
		return 
	}

	refreshToken, err := cfg.dbQueries.GetRefreshToken(r.Context(), token)

	if err != nil{
		respondWithError(w, 401, "unauthorized user")
		return
	}

	if   time.Now().After(refreshToken.ExpiresAt){
		respondWithError(w, 401, "session expired")
		return
	}

	if   refreshToken.RevokedAt.Valid {
		respondWithError(w, 401, "session revoked")
		return
	}

	// user, err := cfg.dbQueries.GetUserFromRefreshToken(r.Context(), refreshToken.Token)

	// if err != nil{
	// 	respondWithError(w, 401, "unauthorized user")
	// 	return
	// }

	newJWTtoken , err := auth.MakeJWT(refreshToken.UserID, cfg.JWTSecret, time.Hour)

	
	if err != nil{
		respondWithError(w, 500, err.Error())
		return
	}

	type responseBody struct{
		Token string `json:"token"` 
	}

	respondWithJSON(w, 200, responseBody{Token: newJWTtoken})



}


func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request){

	type requestBody struct{
		Email string `json:"email"`
		Password string `json:"password"`
	}

	req := requestBody{}

	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&req)

	if err != nil{
		respondWithError(w, 500, "internal server error")
		return
	}

	 hash, err := auth.HashPassword(req.Password)

	 	if err != nil{
		respondWithError(w, 500, "internal server error")
		return
	}

	dbUser, errr := cfg.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email: req.Email, 
		HashedPassword: hash,
	})

	if errr != nil{
		respondWithError(w, 400, "bad request")
		return
	}
	

	user := User{
		ID: dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email: dbUser.Email,
		IsChirpyRed: dbUser.IsChirpyRed,
	}

	respondWithJSON(w, 201, user)

}

func (cfg *apiConfig) getUser(w http.ResponseWriter, r *http.Request){

	type requestBody struct{
		Email string `json:"email"`
		Password string `json:"password"`
		
	}

	type response struct{
		User
		AccessToken string `json:"token"`
		RefreshToken string `json:"refresh_token"`

	}

	req := requestBody{}

	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&req)

	if err != nil{
		respondWithError(w, 500, "internal server error")
		return
	}

	dbUser, err := cfg.dbQueries.GetUserByEmail(r.Context(), req.Email)



	if err != nil{
		respondWithError(w, 401, "invalid user or password")
		return
	}

	ok, err := auth.CheckPasswordHash(req.Password, dbUser.HashedPassword)


	if  !ok{
		respondWithError(w, 401, "invalid username or password")
		return
	}
	


	token, err  := auth.MakeJWT(dbUser.ID, cfg.JWTSecret, time.Hour)


	 if err != nil {
	respondWithError(w, 500, "couldn't create access token")
	return
	}

	refreshToken := auth.MakeRefreshToken()

	if refreshToken == ""{
	respondWithError(w, 500, "couldn't create  refresh token")
	return	
	}

	_, refreshTokenErr := cfg.dbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token: refreshToken,
		UserID: dbUser.ID,
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour), // 60 days
	})
 
	 if refreshTokenErr  != nil {
	respondWithError(w, 500, "couldn't store refresh token")
	return
		}


	user := User{
		ID: dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email: dbUser.Email,
		IsChirpyRed: dbUser.IsChirpyRed,
	}

	resp := response{
		User: user, 
		AccessToken: token,
		RefreshToken: refreshToken,

	}
	

	

	respondWithJSON(w, 200, resp)

}

func (cfg *apiConfig) createChirp(w http.ResponseWriter, r *http.Request){

	type Params struct{
		Body string `json:"body"`
		

	}

	 token, tokenError := auth.GetBearerToken(r.Header)

	 if tokenError != nil{
		respondWithError(w, 401, "no token or malformed token")
		return 
	 }

	 PayloadUserID, JWTerr := auth.ValidateJWT(token, cfg.JWTSecret)

	 if JWTerr != nil{
		respondWithError(w, 401, "invalid jwt")
		return 
	 }

	params := Params{}

	

	err := json.NewDecoder(r.Body).Decode(&params)

	if err != nil{
		log.Println(err)
		respondWithError(w, 500, "internal server error")
		return
	}

	

	if len(params.Body) > 140{
		log.Println(err)
		respondWithJSON(w, 400, "too long")
		return
	}


	words := strings.Split(strings.ToLower(params.Body), " ")

	clean := ""
	for _, word := range words{

		if word == "kerfuffle" || word == "sharbert" || word == "fornax" {
			clean += "**** " 
		} else{
			clean += word + " "
		}
		
	}

	chirp, err:= cfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body: clean,
			UserID: PayloadUserID,
		})
	
		if err != nil{
			log.Println(err)
			respondWithError(w, 500, "internal server error")
			return
		}


		chirpSt := Chirp{
		ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
	}	
	respondWithJSON(w, 201, chirpSt)

}

func (cfg *apiConfig) GetAllChirps(w http.ResponseWriter, r *http.Request){

	var chirps []database.Chirp

	var DBerr error

	authorID := r.URL.Query().Get("author_id")

	sortQuery := r.URL.Query().Get("sort")



	if authorID == ""{


		chirps , DBerr = cfg.dbQueries.GetAllChirps(r.Context())

		
	}else{

		userID, err := uuid.Parse(authorID)

		if err != nil{

			respondWithError(w, 500, "server error")
			return
		}

		chirps, DBerr = cfg.dbQueries.GetChirpsByAuthor(r.Context(), userID)




	}
	

	if DBerr != nil{
			log.Println(DBerr)
			respondWithError(w, 500, "internal server error")
			return
		}

	res := []Chirp{}
	
	for _, chirp := range chirps{

		res = append(res, Chirp{
			ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
		})
	}

	if sortQuery == "desc"{
		sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.After(res[j].CreatedAt)})
	}
	
	
	respondWithJSON(w, 200, res)

	

}

func (cfg *apiConfig)GetChirp(w http.ResponseWriter, r *http.Request){

	chirpID, err := uuid.Parse(r.PathValue("chirpID"))

	if err != nil{
			log.Println(err)
			respondWithError(w, 400, "no chirp with given id")
			return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(),chirpID )

	
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "chirp not found")
			return
		}

		log.Println(err)
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	chirpSt := Chirp{
		ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
	}	

	respondWithJSON(w, 200, chirpSt)
}

func (cfg *apiConfig) deleteChirp(w http.ResponseWriter, r *http.Request){

	chirpID, err := uuid.Parse(r.PathValue("chirpID"))

	if err != nil{
		respondWithError(w, 400, "no chirp id")
		return
	}

	token, err := auth.GetBearerToken(r.Header)

	if err != nil{
		respondWithError(w, 401, "missing token")
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.JWTSecret)

	if err != nil{
		respondWithError(w, 401, "unauthorized user")
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(), chirpID)

	if err != nil{
		respondWithError(w, 404, "no chirp")
		return
	}

	if chirp.UserID != userID {
		respondWithError(w, 403, "forbidden for you")
		return
	}

	err = cfg.dbQueries.DeleteChirp(r.Context(), chirpID)

	if err != nil{
		respondWithError(w, 500, "cant delete")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
