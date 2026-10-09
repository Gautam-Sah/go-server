package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/golang-jwt/jwt/v5"
)


func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error){
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy-access",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject: userID.String(),
	})

	ss, err := token.SignedString([]byte(tokenSecret))

	if err == nil{
		fmt.Println(ss)
	}
	

	return ss, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error){

	claims := jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error){
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(token.Claims.(*jwt.RegisteredClaims).Subject) 

	if err != nil {
		return uuid.Nil, err
	}

	fmt.Println(userID)
	return userID, nil
}


// func GetBearerToken(headers http.Header) (string, error){
// 	token := headers.Get("Authorization")

// 	if token == "" {
// 		return "", fmt.Errorf("no token")
// 	}
// 	tokenString := strings.Split(token, " ")

// 	if tokenString[0] != "Bearer" {
//     return "", fmt.Errorf("no token")
// }

// 	if len(tokenString) != 2{
// 		return "", fmt.Errorf("no token")
// 	}

// 	return tokenString[1], nil

// }

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")

	if authHeader == "" {
		return "", fmt.Errorf("authorization header missing")
	}

	const prefix = "Bearer "

	if !strings.HasPrefix(authHeader, prefix) {
		return "", fmt.Errorf("invalid authorization header")
	}

	return strings.TrimPrefix(authHeader, prefix), nil
}

func MakeRefreshToken() string{

	b := make([]byte, 32)

	_, err := rand.Read(b)

	if err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}


func GetAPIKey(headers http.Header) (string, error){
	authHeader := headers.Get("Authorization")

	if authHeader == "" {
		return "", fmt.Errorf("authorization header missing")
	}

	const prefix = "ApiKey "

	if !strings.HasPrefix(authHeader, prefix) {
		return "", fmt.Errorf("invalid authorization header")
	}

	return strings.TrimPrefix(authHeader, prefix), nil
}