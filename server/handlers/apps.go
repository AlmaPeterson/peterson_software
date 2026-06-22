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

type Release struct {
	ID       int64  `json:"id"`
	Platform string `json:"platform"`
	Filename string `json:"filename"`
	FileSize int64  `json:"file_size"`
}

type App struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	Releases    []Release `json:"releases"`
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

// detectPlatform infers the target platform from a file's extension so admins
// don't have to tag each upload manually.
func detectPlatform(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".apk", ".aab":
		return "Android"
	case ".ipa":
		return "iOS"
	case ".exe", ".msi":
		return "Windows"
	case ".dmg", ".pkg":
		return "Mac"
	case ".deb", ".rpm", ".appimage":
		return "Linux"
	default:
		return "Other"
	}
}

func loadReleases() (map[int64][]Release, error) {
	releasesByApp := map[int64][]Release{}
	rows, err := db.DB.Query("SELECT id, app_id, platform, filename, file_size FROM releases ORDER BY platform")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rel Release
		var appID int64
		if err := rows.Scan(&rel.ID, &appID, &rel.Platform, &rel.Filename, &rel.FileSize); err != nil {
			return nil, err
		}
		releasesByApp[appID] = append(releasesByApp[appID], rel)
	}
	return releasesByApp, nil
}

func ListApps(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserKey).(*middleware.Claims)
	isAuthed := claims != nil

	var rows *sql.Rows
	var err error

	// Public users only see public apps; logged-in users see all
	if isAuthed {
		rows, err = db.DB.Query("SELECT id, name, slug, description, version, is_public, created_at FROM apps ORDER BY created_at DESC")
	} else {
		rows, err = db.DB.Query("SELECT id, name, slug, description, version, is_public, created_at FROM apps WHERE is_public=1 ORDER BY created_at DESC")
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
		rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.Version, &isPublicInt, &a.CreatedAt)
		a.IsPublic = isPublicInt == 1
		a.Releases = []Release{}
		apps = append(apps, a)
	}

	releasesByApp, err := loadReleases()
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	for i := range apps {
		if rels, ok := releasesByApp[apps[i].ID]; ok {
			apps[i].Releases = rels
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

func GetApp(w http.ResponseWriter, r *http.Request, slug string) {
	claims, _ := r.Context().Value(middleware.UserKey).(*middleware.Claims)
	isAuthed := claims != nil

	var a App
	var isPublicInt int
	err := db.DB.QueryRow(
		"SELECT id, name, slug, description, version, is_public, created_at FROM apps WHERE slug=?", slug,
	).Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.Version, &isPublicInt, &a.CreatedAt)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}
	a.IsPublic = isPublicInt == 1
	if !a.IsPublic && !isAuthed {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	rows, err := db.DB.Query("SELECT id, platform, filename, file_size FROM releases WHERE app_id=? ORDER BY platform", a.ID)
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	a.Releases = []Release{}
	for rows.Next() {
		var rel Release
		rows.Scan(&rel.ID, &rel.Platform, &rel.Filename, &rel.FileSize)
		a.Releases = append(a.Releases, rel)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

// CreateApp creates a new software entry with no files yet. Files are added
// afterward via UploadChunk — splitting "create" from "upload" lets the
// (potentially large, slow) file transfer happen as a series of small
// requests instead of one long one, which is what made uploads prone to
// timing out or getting reset by intermediate proxies.
func CreateApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		IsPublic    bool   `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Version == "" {
		http.Error(w, "name and version are required", http.StatusBadRequest)
		return
	}

	isPublic := 0
	if req.IsPublic {
		isPublic = 1
	}
	slug := slugify(req.Name)

	res, err := db.DB.Exec(
		"INSERT INTO apps (name, slug, description, version, is_public) VALUES (?,?,?,?,?)",
		req.Name, slug, req.Description, req.Version, isPublic,
	)
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "slug": slug})
}

// UploadChunk appends one chunk of a file to a temporary in-progress upload,
// keyed by a client-generated uploadId. The client sends chunks sequentially;
// once chunkIndex is the last one, the temp file is finalized into a release
// for the given app. Each request only carries a few MB, so it stays well
// under any reverse proxy's timeout or body-size limit regardless of how
// large the overall file is or how slow the connection is.
func UploadChunk(w http.ResponseWriter, r *http.Request, appID int64) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil { // chunks are small; this is generous headroom
		http.Error(w, "Failed to read chunk: "+err.Error(), http.StatusBadRequest)
		return
	}

	uploadID := r.FormValue("uploadId")
	filename := r.FormValue("filename")
	chunkIndex, errIdx := strconv.Atoi(r.FormValue("chunkIndex"))
	totalChunks, errTotal := strconv.Atoi(r.FormValue("totalChunks"))
	if uploadID == "" || filename == "" || errIdx != nil || errTotal != nil || chunkIndex < 0 || totalChunks <= 0 {
		http.Error(w, "Invalid chunk metadata", http.StatusBadRequest)
		return
	}

	chunk, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "Missing chunk data", http.StatusBadRequest)
		return
	}
	defer chunk.Close()

	var slug, version string
	if err := db.DB.QueryRow("SELECT slug, version FROM apps WHERE id=?", appID).Scan(&slug, &version); err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	tempPath := filepath.Join("releases", ".tmp-"+uploadID)
	f, err := os.OpenFile(tempPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Could not open temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_, copyErr := io.Copy(f, chunk)
	f.Close()
	if copyErr != nil {
		os.Remove(tempPath)
		http.Error(w, "Failed to write chunk: "+copyErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if chunkIndex < totalChunks-1 {
		json.NewEncoder(w).Encode(map[string]interface{}{"received": chunkIndex})
		return
	}

	// Last chunk — finalize into a release.
	info, statErr := os.Stat(tempPath)
	if statErr != nil {
		http.Error(w, "Upload session not found", http.StatusBadRequest)
		return
	}
	finalFilename := fmt.Sprintf("%s-%s-%s", slug, version, filename)
	finalPath := filepath.Join("releases", finalFilename)
	if err := os.Rename(tempPath, finalPath); err != nil {
		http.Error(w, "Could not finalize upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	platform := detectPlatform(filename)
	res, err := db.DB.Exec(
		"INSERT INTO releases (app_id, platform, filename, file_size) VALUES (?,?,?,?)",
		appID, platform, finalFilename, info.Size(),
	)
	if err != nil {
		os.Remove(finalPath)
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": id, "platform": platform, "filename": finalFilename, "file_size": info.Size(),
	})
}

func DownloadApp(w http.ResponseWriter, r *http.Request, slug, platform string) {
	var appID int64
	var isPublicInt int
	err := db.DB.QueryRow("SELECT id, is_public FROM apps WHERE slug=?", slug).Scan(&appID, &isPublicInt)
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

	var filename string
	err = db.DB.QueryRow(
		"SELECT filename FROM releases WHERE app_id=? AND LOWER(platform)=LOWER(?)", appID, platform,
	).Scan(&filename)
	if err != nil {
		http.Error(w, "No file available for that platform", http.StatusNotFound)
		return
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

	rows, err := db.DB.Query("SELECT filename FROM releases WHERE app_id=?", id)
	if err == nil {
		var filenames []string
		for rows.Next() {
			var f string
			rows.Scan(&f)
			filenames = append(filenames, f)
		}
		rows.Close()
		for _, f := range filenames {
			os.Remove(filepath.Join("releases", f))
		}
	}

	db.DB.Exec("DELETE FROM releases WHERE app_id=?", id)
	db.DB.Exec("DELETE FROM apps WHERE id=?", id)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteRelease removes a single platform-specific file, leaving the rest of
// the software's files intact.
func DeleteRelease(w http.ResponseWriter, r *http.Request, id int64) {
	var filename string
	err := db.DB.QueryRow("SELECT filename FROM releases WHERE id=?", id).Scan(&filename)
	if err != nil {
		http.Error(w, "Release not found", http.StatusNotFound)
		return
	}
	db.DB.Exec("DELETE FROM releases WHERE id=?", id)
	if filename != "" {
		os.Remove(filepath.Join("releases", filename))
	}
	w.WriteHeader(http.StatusNoContent)
}
