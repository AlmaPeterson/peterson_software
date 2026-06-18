package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"peterson-software/db"
	"peterson-software/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" || body.Email == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	_, err = db.DB.Exec(
		"INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, 'pending')",
		body.Username, body.Email, string(hash),
	)
	if err != nil {
		http.Error(w, "Username or email already taken", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful. Awaiting admin approval."})
}

func Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	var id int64
	var hash, role string
	err := db.DB.QueryRow(
		"SELECT id, password_hash, role FROM users WHERE username=?", body.Username,
	).Scan(&id, &hash, &role)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if role == "pending" {
		http.Error(w, "Account pending admin approval", http.StatusForbidden)
		return
	}
	claims := &middleware.Claims{
		UserID:   id,
		Username: body.Username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(middleware.JWTSecret)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    signed,
		"username": body.Username,
		"role":     role,
	})
}

func Me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(middleware.UserKey).(*middleware.Claims)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
	})
}