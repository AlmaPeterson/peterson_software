package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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

func saveUploadedFile(slug, version string, fh *multipart.FileHeader) (filename string, size int64, err error) {
	filename = fmt.Sprintf("%s-%s-%s", slug, version, fh.Filename)
	destPath := filepath.Join("releases", filename)

	src, err := fh.Open()
	if err != nil {
		return "", 0, err
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return "", 0, err
	}
	defer dest.Close()

	size, err = io.Copy(dest, src)
	if err != nil {
		// Don't leave a truncated file on disk if the upload was interrupted.
		os.Remove(destPath)
		return "", 0, err
	}
	return filename, size, err
}

func UploadApp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(500 << 20); err != nil { // 500MB
		http.Error(w, "Failed to read upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	version := r.FormValue("version")
	description := r.FormValue("description")
	isPublicStr := r.FormValue("is_public")
	isPublic := 1
	if isPublicStr == "false" || isPublicStr == "0" {
		isPublic = 0
	}

	if name == "" || version == "" {
		http.Error(w, "name and version are required", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "At least one file is required", http.StatusBadRequest)
		return
	}

	slug := slugify(name)

	res, err := db.DB.Exec(
		"INSERT INTO apps (name, slug, description, version, is_public) VALUES (?,?,?,?,?)",
		name, slug, description, version, isPublic,
	)
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	appID, _ := res.LastInsertId()

	var saved []string
	var failed []string
	for _, fh := range files {
		filename, size, err := saveUploadedFile(slug, version, fh)
		if err != nil {
			log.Printf("upload: failed to save %q for app %d: %v", fh.Filename, appID, err)
			failed = append(failed, fh.Filename)
			continue
		}
		platform := detectPlatform(fh.Filename)
		_, err = db.DB.Exec(
			"INSERT INTO releases (app_id, platform, filename, file_size) VALUES (?,?,?,?)",
			appID, platform, filename, size,
		)
		if err != nil {
			log.Printf("upload: failed to record release %q for app %d: %v", filename, appID, err)
			os.Remove(filepath.Join("releases", filename))
			failed = append(failed, fh.Filename)
			continue
		}
		saved = append(saved, filename)
	}

	if len(saved) == 0 {
		db.DB.Exec("DELETE FROM apps WHERE id=?", appID)
		http.Error(w, "Could not save any uploaded files", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": appID, "files": saved, "failed": failed})
}

// UploadAppFile adds one more platform-specific file to an existing software entry.
func UploadAppFile(w http.ResponseWriter, r *http.Request, appID int64) {
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, "Failed to read upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	var slug, version string
	err := db.DB.QueryRow("SELECT slug, version FROM apps WHERE id=?", appID).Scan(&slug, &version)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File upload error", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("%s-%s-%s", slug, version, header.Filename)
	destPath := filepath.Join("releases", filename)
	dest, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Could not save file", http.StatusInternalServerError)
		return
	}
	defer dest.Close()
	size, err := io.Copy(dest, file)
	if err != nil {
		os.Remove(destPath)
		http.Error(w, "Upload was interrupted before it finished", http.StatusBadRequest)
		return
	}

	platform := detectPlatform(header.Filename)
	res, err := db.DB.Exec(
		"INSERT INTO releases (app_id, platform, filename, file_size) VALUES (?,?,?,?)",
		appID, platform, filename, size,
	)
	if err != nil {
		os.Remove(destPath)
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "platform": platform, "filename": filename})
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
