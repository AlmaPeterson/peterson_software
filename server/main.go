package main

import (
	"log"
	"net/http"
	"os"
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

	// Download (optional auth — private files require auth)
	mux.Handle("/api/apps/", middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			handlers.DownloadApp(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	// Admin routes
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/api/admin/apps/upload", handlers.UploadApp)
	adminMux.HandleFunc("/api/admin/apps/delete/", handlers.DeleteApp)
	adminMux.HandleFunc("/api/admin/users", handlers.ListUsers)
	adminMux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/role") {
			handlers.UpdateUserRole(w, r)
		} else if r.Method == http.MethodDelete {
			handlers.DeleteUser(w, r)
		}
	})
	mux.Handle("/api/admin/", middleware.Auth(middleware.AdminOnly(adminMux)))

	// Serve built React frontend
	fs := http.FileServer(http.Dir("../frontend/dist"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// SPA fallback
		if _, err := os.Stat("../frontend/dist" + r.URL.Path); os.IsNotExist(err) {
			http.ServeFile(w, r, "../frontend/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Println("Peterson Software running on http://localhost:443")
	log.Fatal(http.ListenAndServe(":443", corsMiddleware(mux)))
}