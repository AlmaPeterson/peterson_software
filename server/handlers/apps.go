package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"peterson-software/db"
	"peterson-software/middleware"
)

type App struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Platform    string    `json:"platform"`
	Version     string    `json:"version"`
	Filename    string    `json:"filename"`
	FileSize    int64     `json:"file_size"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ListApps(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserKey).(*middleware.Claims)
	isAuthed := claims != nil

	var rows *sql.Rows // This is intentionally using the named import below
	var err error

	// Public users only see public apps; logged-in users see all
	if isAuthed {
		rows, err = db.DB.Query("SELECT id, name, slug, description, platform, version, filename, file_size, is_public, created_at FROM apps ORDER BY created_at DESC")
	} else {
		rows, err = db.DB.Query("SELECT id, name, slug, description, platform, version, filename, file_size, is_public, created_at FROM apps WHERE is_public=1 ORDER BY created_at DESC")
	}
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	apps := []App{}
	for rows.Next() {
		var a App
		var isPublicInt int
		rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.Platform, &a.Version, &a.Filename, &a.FileSize, &isPublicInt, &a.CreatedAt)
		a.IsPublic = isPublicInt == 1
		apps = append(apps, a)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

func UploadApp(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(500 << 20) // 500MB

	name := r.FormValue("name")
	platform := r.FormValue("platform")
	version := r.FormValue("version")
	description := r.FormValue("description")
	isPublicStr := r.FormValue("is_public")
	isPublic := 1
	if isPublicStr == "false" || isPublicStr == "0" {
		isPublic = 0
	}

	if name == "" || platform == "" || version == "" {
		http.Error(w, "name, platform and version are required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File upload error", http.StatusBadRequest)
		return
	}
	defer file.Close()

	slug := slugify(name)
	filename := fmt.Sprintf("%s-%s-%s", slug, version, header.Filename)
	destPath := filepath.Join("releases", filename)

	dest, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Could not save file", http.StatusInternalServerError)
		return
	}
	defer dest.Close()
	size, _ := io.Copy(dest, file)

	res, err := db.DB.Exec(
		"INSERT INTO apps (name, slug, description, platform, version, filename, file_size, is_public) VALUES (?,?,?,?,?,?,?,?)",
		name, slug, description, platform, version, filename, size, isPublic,
	)
	if err != nil {
		os.Remove(destPath)
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "filename": filename})
}

func DownloadApp(w http.ResponseWriter, r *http.Request) {
	// Extract slug from path: /api/apps/{slug}/download
	parts := strings.Split(r.URL.Path, "/")
	slug := parts[len(parts)-2]

	var filename string
	var isPublicInt int
	err := db.DB.QueryRow("SELECT filename, is_public FROM apps WHERE slug=?", slug).Scan(&filename, &isPublicInt)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	if isPublicInt == 0 {
		claims, ok := r.Context().Value(middleware.UserKey).(*middleware.Claims)
		if !ok || claims == nil {
			http.Error(w, "Login required to download this file", http.StatusUnauthorized)
			return
		}
	}

	filePath := filepath.Join("releases", filename)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, filePath)
}

func DeleteApp(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	idStr := parts[len(parts)-1]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var filename string
	db.DB.QueryRow("SELECT filename FROM apps WHERE id=?", id).Scan(&filename)
	db.DB.Exec("DELETE FROM apps WHERE id=?", id)
	if filename != "" {
		os.Remove(filepath.Join("releases", filename))
	}
	w.WriteHeader(http.StatusNoContent)
}