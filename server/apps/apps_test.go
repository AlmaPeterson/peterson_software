package apps

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestStore(t *testing.T) *Store {
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

	store, err := NewStoreWithDirs(db, filepath.Join(t.TempDir(), "icons"), filepath.Join(t.TempDir(), "releases"))
	if err != nil {
		t.Fatalf("NewStoreWithDirs: %v", err)
	}
	return store
}

func TestCreate_RequiresNameAndVersion(t *testing.T) {
	s := newTestStore(t)
	cases := []CreateInput{
		{Name: "", Version: "1.0"},
		{Name: "Foo", Version: ""},
	}
	for _, in := range cases {
		if _, err := s.Create(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Create(%+v) = %v, want ErrInvalid", in, err)
		}
	}
}

func TestCreate_DuplicateSlug(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(CreateInput{Name: "Foo App", Version: "1.0"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := s.Create(CreateInput{Name: "foo-app", Version: "2.0"})
	if !errors.Is(err, ErrSlugTaken) {
		t.Errorf("second Create = %v, want ErrSlugTaken", err)
	}
}

func TestCreate_Success(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "Kinetic Flow", Version: "1.0", Description: "desc", IsPublic: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if app.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if app.Slug != "kinetic-flow" {
		t.Errorf("Slug = %q, want %q", app.Slug, "kinetic-flow")
	}
	if !app.IsPublic {
		t.Error("expected IsPublic true")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get = %v, want ErrNotFound", err)
	}
}

func TestGet_ReturnsReleases(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "Field Clock", Version: "2.1.0"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	tempPath := writeTempUpload(t, "app.apk")
	if _, err := s.AddRelease(app.ID, tempPath, "app.apk", 1234); err != nil {
		t.Fatalf("AddRelease: %v", err)
	}

	got, err := s.Get(app.Slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Releases) != 1 {
		t.Fatalf("Releases = %v, want 1 entry", got.Releases)
	}
	if got.Releases[0].Platform != "Android" {
		t.Errorf("Platform = %q, want Android", got.Releases[0].Platform)
	}
}

func TestList_OnlyPublic(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(CreateInput{Name: "Public App", Version: "1.0", IsPublic: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(CreateInput{Name: "Private App", Version: "1.0", IsPublic: false}); err != nil {
		t.Fatal(err)
	}

	publicOnly, err := s.List(true)
	if err != nil {
		t.Fatalf("List(true): %v", err)
	}
	if len(publicOnly) != 1 || publicOnly[0].Name != "Public App" {
		t.Errorf("List(true) = %+v, want only Public App", publicOnly)
	}

	all, err := s.List(false)
	if err != nil {
		t.Fatalf("List(false): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List(false) = %d apps, want 2", len(all))
	}
}

func TestUpdate_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.Update(999, UpdateInput{Name: "X", Version: "1.0"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Update = %v, want ErrNotFound", err)
	}
}

func TestUpdate_SlugUnchangedOnRename(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "Old Name", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(app.ID, UpdateInput{Name: "New Name", Version: "1.1"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.Get(app.Slug) // old slug
	if err != nil {
		t.Fatalf("Get by original slug should still work: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want New Name", got.Name)
	}
	if got.Slug != app.Slug {
		t.Errorf("Slug changed from %q to %q", app.Slug, got.Slug)
	}
}

func TestSetIcon_RejectsBadExtension(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "App", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SetIcon(app.ID, ".svg", strings.NewReader("<svg/>"))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("SetIcon(.svg) = %v, want ErrInvalid", err)
	}
}

func TestSetIcon_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.SetIcon(999, ".png", strings.NewReader("data"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("SetIcon = %v, want ErrNotFound", err)
	}
}

func TestSetIcon_Success(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "App", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	url, err := s.SetIcon(app.ID, ".png", strings.NewReader("fake-png-bytes"))
	if err != nil {
		t.Fatalf("SetIcon: %v", err)
	}
	wantURL := "/icons/app-icon.png"
	if url != wantURL {
		t.Errorf("IconURL = %q, want %q", url, wantURL)
	}
	if _, err := os.Stat(filepath.Join(s.iconsDir, "app-icon.png")); err != nil {
		t.Errorf("icon file not written: %v", err)
	}

	got, err := s.Get(app.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.IconURL != wantURL {
		t.Errorf("Get().IconURL = %q, want %q", got.IconURL, wantURL)
	}
}

func TestAddRelease_NotFound(t *testing.T) {
	s := newTestStore(t)
	tempPath := writeTempUpload(t, "x.apk")
	_, err := s.AddRelease(999, tempPath, "x.apk", 10)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("AddRelease = %v, want ErrNotFound", err)
	}
	if _, statErr := os.Stat(tempPath); statErr != nil {
		t.Error("temp file should be left in place when the app doesn't exist")
	}
}

func TestDelete_RemovesAppReleasesAndFiles(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "Doomed App", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetIcon(app.ID, ".png", strings.NewReader("icon")); err != nil {
		t.Fatal(err)
	}
	tempPath := writeTempUpload(t, "x.apk")
	rel, err := s.AddRelease(app.ID, tempPath, "x.apk", 10)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(app.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := s.Get(app.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if _, statErr := os.Stat(filepath.Join(s.releasesDir, rel.Filename)); !os.IsNotExist(statErr) {
		t.Error("release file should have been removed")
	}
	if _, statErr := os.Stat(filepath.Join(s.iconsDir, "doomed-app-icon.png")); !os.IsNotExist(statErr) {
		t.Error("icon file should have been removed")
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.Delete(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete = %v, want ErrNotFound", err)
	}
}

func TestDeleteRelease(t *testing.T) {
	s := newTestStore(t)
	app, err := s.Create(CreateInput{Name: "App", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	tempPath := writeTempUpload(t, "x.apk")
	rel, err := s.AddRelease(app.ID, tempPath, "x.apk", 10)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteRelease(rel.ID); err != nil {
		t.Fatalf("DeleteRelease: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(s.releasesDir, rel.Filename)); !os.IsNotExist(statErr) {
		t.Error("release file should have been removed")
	}
	if err := s.DeleteRelease(rel.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second DeleteRelease = %v, want ErrNotFound", err)
	}
}

// writeTempUpload simulates a finished chunked upload: a file sitting
// outside the releases directory, ready for AddRelease to move into place.
func writeTempUpload(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".tmp-"+name)
	if err := os.WriteFile(path, []byte("fake-binary-contents"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
