package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"peterson-software/db"
	"peterson-software/handlers"
	"peterson-software/middleware"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	os.MkdirAll("releases", 0755)
	db.Init("../data.db")

	mux := http.NewServeMux()

	// Auth routes
	mux.HandleFunc("/api/auth/register", handlers.Register)
	mux.HandleFunc("/api/auth/login", handlers.Login)
	mux.Handle("/api/auth/me", middleware.Auth(http.HandlerFunc(handlers.Me)))

	// App listing (optional auth — shows private apps if logged in)
	mux.Handle("/api/apps", middleware.OptionalAuth(http.HandlerFunc(handlers.ListApps)))

	// App detail + per-platform download (optional auth — private apps require auth)
	mux.Handle("/api/apps/", middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/apps/"), "/"), "/")
		switch len(rest) {
		case 1:
			handlers.GetApp(w, r, rest[0])
		case 3:
			if rest[1] == "download" {
				handlers.DownloadApp(w, r, rest[0], rest[2])
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	// Admin routes
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/api/admin/apps", handlers.CreateApp)
	adminMux.HandleFunc("/api/admin/apps/delete/", handlers.DeleteApp)
	adminMux.HandleFunc("/api/admin/apps/", func(w http.ResponseWriter, r *http.Request) {
		// /api/admin/apps/{id}/files/chunk — append one chunk of a file upload to an app
		rest := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/apps/"), "/"), "/")
		if len(rest) == 3 && rest[1] == "files" && rest[2] == "chunk" {
			id, err := strconv.ParseInt(rest[0], 10, 64)
			if err != nil {
				http.Error(w, "Invalid ID", http.StatusBadRequest)
				return
			}
			handlers.UploadChunk(w, r, id)
			return
		}
		http.NotFound(w, r)
	})
	adminMux.HandleFunc("/api/admin/releases/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/releases/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		handlers.DeleteRelease(w, r, id)
	})
	adminMux.HandleFunc("/api/admin/redeploy", handlers.Redeploy)
	adminMux.HandleFunc("/api/admin/users", handlers.ListUsers)
	adminMux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/role") {
			handlers.UpdateUserRole(w, r)
		} else if r.Method == http.MethodDelete {
			handlers.DeleteUser(w, r)
		}
	})
	mux.Handle("/api/admin/", middleware.Auth(middleware.AdminOnly(adminMux)))

	// Serve static frontend
	fs := http.FileServer(http.Dir("../frontend/dist"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Force revalidation so deployed HTML/CSS/JS changes are never served
		// stale from a browser cache (there's no cache-busting/versioning on
		// these filenames, so without this a redeploy can silently appear to
		// do nothing client-side).
		w.Header().Set("Cache-Control", "no-cache")
		// SPA fallback
		if _, err := os.Stat("../frontend/dist" + r.URL.Path); os.IsNotExist(err) {
			http.ServeFile(w, r, "../frontend/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Println("Peterson Software running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", corsMiddleware(mux)))
}