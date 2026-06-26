package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"peterson-software/apps"
	"peterson-software/middleware"
)

// AppHandlers exposes the apps.Store through HTTP. It holds no state of its
// own beyond the Store, so tests construct one with a Store of their own —
// no shared globals, no real database required.
type AppHandlers struct {
	Store *apps.Store
}

func writeAppsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, apps.ErrNotFound):
		http.Error(w, "Not found", http.StatusNotFound)
	case errors.Is(err, apps.ErrInvalid):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, apps.ErrSlugTaken):
		http.Error(w, "An app with this name already exists", http.StatusConflict)
	default:
		http.Error(w, "Server error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (h *AppHandlers) ListApps(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserKey).(*middleware.Claims)
	onlyPublic := claims == nil

	result, err := h.Store.List(onlyPublic)
	if err != nil {
		writeAppsError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *AppHandlers) GetApp(w http.ResponseWriter, r *http.Request, slug string) {
	claims, _ := r.Context().Value(middleware.UserKey).(*middleware.Claims)
	isAuthed := claims != nil

	app, err := h.Store.Get(slug)
	if err != nil {
		writeAppsError(w, err)
		return
	}
	if !app.IsPublic && !isAuthed {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

// CreateApp creates a new software entry with no files yet. Files are added
// afterward via UploadChunk — splitting "create" from "upload" lets the
// (potentially large, slow) file transfer happen as a series of small
// requests instead of one long one, which is what made uploads prone to
// timing out or getting reset by intermediate proxies.
func (h *AppHandlers) CreateApp(w http.ResponseWriter, r *http.Request) {
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

	app, err := h.Store.Create(apps.CreateInput{
		Name:        req.Name,
		Version:     req.Version,
		Description: req.Description,
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		writeAppsError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": app.ID, "slug": app.Slug})
}

func (h *AppHandlers) UpdateApp(w http.ResponseWriter, r *http.Request, appID int64) {
	if r.Method != http.MethodPut {
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

	err := h.Store.Update(appID, apps.UpdateInput{
		Name:        req.Name,
		Version:     req.Version,
		Description: req.Description,
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		writeAppsError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AppHandlers) UploadIcon(w http.ResponseWriter, r *http.Request, appID int64) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to read upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("icon")
	if err != nil {
		http.Error(w, "Missing icon file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	iconURL, err := h.Store.SetIcon(appID, ext, file)
	if err != nil {
		writeAppsError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"icon_url": iconURL})
}

// UploadChunk appends one chunk of a file to a temporary in-progress upload,
// keyed by a client-generated uploadId. The client may send the whole file as
// a single chunk (totalChunks=1) or split it into N chunks sent sequentially;
// once chunkIndex is the last one, the temp file is finalized into a release
// for the given app via Store.AddRelease. Starting with a single large request
// and falling back to smaller chunks on failure is handled client-side.
func (h *AppHandlers) UploadChunk(w http.ResponseWriter, r *http.Request, appID int64) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB in-memory; larger bodies spill to disk
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

	chunkHash := r.FormValue("chunkHash") // optional SHA-256 hex sent by client

	chunk, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "Missing chunk data", http.StatusBadRequest)
		return
	}
	defer chunk.Close()

	tempPath := h.Store.TempUploadPath(uploadID)
	f, err := os.OpenFile(tempPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Could not open temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(f, io.TeeReader(chunk, hasher))
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tempPath)
		http.Error(w, "Failed to write chunk: "+copyErr.Error(), http.StatusInternalServerError)
		return
	}
	if closeErr != nil {
		os.Remove(tempPath)
		http.Error(w, "Failed to write chunk: "+closeErr.Error(), http.StatusInternalServerError)
		return
	}
	if chunkHash != "" {
		if actual := hex.EncodeToString(hasher.Sum(nil)); actual != chunkHash {
			os.Remove(tempPath)
			http.Error(w, "Chunk integrity check failed", http.StatusUnprocessableEntity)
			return
		}
	}

	info, statErr := os.Stat(tempPath)
	if statErr != nil {
		http.Error(w, "Upload session not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if chunkIndex < totalChunks-1 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"received":      chunkIndex,
			"bytesReceived": info.Size(),
		})
		return
	}

	// Last chunk — finalize into a release.
	rel, err := h.Store.AddRelease(appID, tempPath, filename, info.Size())
	if err != nil {
		writeAppsError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": rel.ID, "platform": rel.Platform, "filename": rel.Filename,
		"file_size": rel.FileSize, "bytesReceived": info.Size(),
	})
}

func (h *AppHandlers) DownloadApp(w http.ResponseWriter, r *http.Request, slug, platform string) {
	app, err := h.Store.Get(slug)
	if err != nil {
		writeAppsError(w, err)
		return
	}

	if !app.IsPublic {
		claims, ok := r.Context().Value(middleware.UserKey).(*middleware.Claims)
		if !ok || claims == nil {
			http.Error(w, "Login required to download this file", http.StatusUnauthorized)
			return
		}
	}

	var filename string
	for _, rel := range app.Releases {
		if strings.EqualFold(rel.Platform, platform) {
			filename = rel.Filename
			break
		}
	}
	if filename == "" {
		http.Error(w, "No file available for that platform", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, h.Store.ReleaseFilePath(filename))
}

func (h *AppHandlers) DeleteApp(w http.ResponseWriter, r *http.Request, appID int64) {
	if err := h.Store.Delete(appID); err != nil && !errors.Is(err, apps.ErrNotFound) {
		writeAppsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AppHandlers) DeleteRelease(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.Store.DeleteRelease(id); err != nil {
		writeAppsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
