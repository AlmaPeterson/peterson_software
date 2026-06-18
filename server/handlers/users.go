package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"peterson-software/db"
)

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

func ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, username, email, role, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt)
		users = append(users, u)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	idStr := parts[len(parts)-2] // /api/admin/users/{id}/role

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	allowed := map[string]bool{"admin": true, "user": true, "pending": true}
	if !allowed[body.Role] {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}
	db.DB.Exec("UPDATE users SET role=? WHERE id=?", body.Role, idStr)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Role updated"})
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	idStr := parts[len(parts)-1]
	db.DB.Exec("DELETE FROM users WHERE id=? AND role!='admin'", idStr)
	w.WriteHeader(http.StatusNoContent)
}