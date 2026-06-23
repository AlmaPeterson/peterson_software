package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"peterson-software/apps"
	"peterson-software/middleware"
)

func newTestHandlers(t *testing.T) *AppHandlers {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
	CREATE TABLE apps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		description TEXT,
		version TEXT NOT NULL,
		is_public INTEGER NOT NULL DEFAULT 1,
		icon_filename TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE releases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id INTEGER NOT NULL,
		platform TEXT NOT NULL,
		filename TEXT NOT NULL,
		file_size INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(app_id) REFERENCES apps(id)
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	store, err := apps.NewStoreWithDirs(db, filepath.Join(t.TempDir(), "icons"), filepath.Join(t.TempDir(), "releases"))
	if err != nil {
		t.Fatalf("NewStoreWithDirs: %v", err)
	}
	return &AppHandlers{Store: store}
}

func withAuth(r *http.Request) *http.Request {
	claims := &middleware.Claims{UserID: 1, Username: "alma", Role: "admin"}
	return r.WithContext(context.WithValue(r.Context(), middleware.UserKey, claims))
}

func createTestApp(t *testing.T, h *AppHandlers, name string, isPublic bool) (id int64, slug string) {
	t.Helper()
	body := strings.NewReader(`{"name":"` + name + `","version":"1.0","is_public":` + strconv.FormatBool(isPublic) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/apps", body)
	rec := httptest.NewRecorder()
	h.CreateApp(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateApp status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID   int64  `json:"id"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode CreateApp response: %v", err)
	}
	return resp.ID, resp.Slug
}

func TestListApps_FiltersPrivateForAnonymous(t *testing.T) {
	h := newTestHandlers(t)
	createTestApp(t, h, "Public One", true)
	createTestApp(t, h, "Private One", false)

	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	rec := httptest.NewRecorder()
	h.ListApps(rec, req)

	var got []apps.App
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Public One" {
		t.Errorf("anonymous ListApps = %+v, want only Public One", got)
	}

	req = withAuth(httptest.NewRequest(http.MethodGet, "/api/apps", nil))
	rec = httptest.NewRecorder()
	h.ListApps(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("authed ListApps returned %d apps, want 2", len(got))
	}
}

func TestCreateApp_MissingFields(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/apps", strings.NewReader(`{"name":""}`))
	rec := httptest.NewRecorder()
	h.CreateApp(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateApp_DuplicateSlugReturns409(t *testing.T) {
	h := newTestHandlers(t)
	createTestApp(t, h, "Same Name", true)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/apps", strings.NewReader(`{"name":"Same Name","version":"2.0"}`))
	rec := httptest.NewRecorder()
	h.CreateApp(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetApp_NotFound(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/apps/missing", nil)
	rec := httptest.NewRecorder()
	h.GetApp(rec, req, "missing")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestGetApp_PrivateRequiresAuth(t *testing.T) {
	h := newTestHandlers(t)
	_, slug := createTestApp(t, h, "Secret App", false)

	req := httptest.NewRequest(http.MethodGet, "/api/apps/"+slug, nil)
	rec := httptest.NewRecorder()
	h.GetApp(rec, req, slug)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous status = %d, want 401", rec.Code)
	}

	req = withAuth(httptest.NewRequest(http.MethodGet, "/api/apps/"+slug, nil))
	rec = httptest.NewRecorder()
	h.GetApp(rec, req, slug)
	if rec.Code != http.StatusOK {
		t.Errorf("authed status = %d, want 200", rec.Code)
	}
}

func TestUpdateApp_NotFound(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/apps/999", strings.NewReader(`{"name":"X","version":"1.0"}`))
	rec := httptest.NewRecorder()
	h.UpdateApp(rec, req, 999)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func multipartIconRequest(t *testing.T, filename string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("icon", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("fake-bytes"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/apps/1/icon", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadIcon_RejectsBadExtension(t *testing.T) {
	h := newTestHandlers(t)
	id, _ := createTestApp(t, h, "Icon App", true)

	req := multipartIconRequest(t, "evil.svg")
	rec := httptest.NewRecorder()
	h.UploadIcon(rec, req, id)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUploadIcon_Success(t *testing.T) {
	h := newTestHandlers(t)
	id, slug := createTestApp(t, h, "Icon App", true)

	req := multipartIconRequest(t, "logo.png")
	rec := httptest.NewRecorder()
	h.UploadIcon(rec, req, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/apps/"+slug, nil)
	getRec := httptest.NewRecorder()
	h.GetApp(getRec, getReq, slug)
	var got apps.App
	json.Unmarshal(getRec.Body.Bytes(), &got)
	if got.IconURL == "" {
		t.Error("expected IconURL to be set after upload")
	}
}

func multipartChunkRequest(t *testing.T, uploadID, filename string, chunkIndex, totalChunks int, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("uploadId", uploadID)
	w.WriteField("filename", filename)
	w.WriteField("chunkIndex", strconv.Itoa(chunkIndex))
	w.WriteField("totalChunks", strconv.Itoa(totalChunks))
	part, err := w.CreateFormFile("chunk", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/apps/1/files/chunk", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadChunk_SingleChunkCreatesReleaseThenDownloads(t *testing.T) {
	h := newTestHandlers(t)
	id, slug := createTestApp(t, h, "Release App", true)

	req := multipartChunkRequest(t, "upload-1", "build.apk", 0, 1, []byte("apk-bytes"))
	rec := httptest.NewRecorder()
	h.UploadChunk(rec, req, id)
	if rec.Code != http.StatusCreated {
		t.Fatalf("UploadChunk status = %d, want 201, body = %s", rec.Code, rec.Body.String())
	}
	var rel apps.Release
	if err := json.Unmarshal(rec.Body.Bytes(), &rel); err != nil {
		t.Fatal(err)
	}
	if rel.Platform != "Android" {
		t.Errorf("Platform = %q, want Android", rel.Platform)
	}

	dlReq := httptest.NewRequest(http.MethodGet, "/api/apps/"+slug+"/download/android", nil)
	dlRec := httptest.NewRecorder()
	h.DownloadApp(dlRec, dlReq, slug, "android")
	if dlRec.Code != http.StatusOK {
		t.Errorf("DownloadApp status = %d, want 200, body = %s", dlRec.Code, dlRec.Body.String())
	}
	if dlRec.Body.String() != "apk-bytes" {
		t.Errorf("downloaded body = %q, want %q", dlRec.Body.String(), "apk-bytes")
	}
}

func TestUploadChunk_MultiChunkAssemblesInOrder(t *testing.T) {
	h := newTestHandlers(t)
	id, slug := createTestApp(t, h, "Multi Chunk App", true)

	rec1 := httptest.NewRecorder()
	h.UploadChunk(rec1, multipartChunkRequest(t, "upload-2", "build.exe", 0, 2, []byte("part1-")), id)
	if rec1.Code != http.StatusOK {
		t.Fatalf("chunk 0 status = %d, want 200, body = %s", rec1.Code, rec1.Body.String())
	}

	rec2 := httptest.NewRecorder()
	h.UploadChunk(rec2, multipartChunkRequest(t, "upload-2", "build.exe", 1, 2, []byte("part2")), id)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("chunk 1 status = %d, want 201, body = %s", rec2.Code, rec2.Body.String())
	}

	dlReq := httptest.NewRequest(http.MethodGet, "/api/apps/"+slug+"/download/windows", nil)
	dlRec := httptest.NewRecorder()
	h.DownloadApp(dlRec, dlReq, slug, "windows")
	if dlRec.Body.String() != "part1-part2" {
		t.Errorf("assembled body = %q, want %q", dlRec.Body.String(), "part1-part2")
	}
}

func TestDeleteApp_IdempotentOnMissing(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/apps/delete/999", nil)
	rec := httptest.NewRecorder()
	h.DeleteApp(rec, req, 999)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestDeleteRelease_NotFound(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/releases/999", nil)
	rec := httptest.NewRecorder()
	h.DeleteRelease(rec, req, 999)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
