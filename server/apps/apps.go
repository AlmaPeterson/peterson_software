// Package apps owns the App and Release data: the SQL, the filesystem
// layout for icons and release files, and the domain invariants around
// slugs, validation, and platform detection. Callers see App and Release as
// already-assembled values; they never see a row or a file path.
package apps

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var (
	ErrNotFound  = errors.New("apps: not found")
	ErrInvalid   = errors.New("apps: invalid input")
	ErrSlugTaken = errors.New("apps: slug already taken")
)

type App struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	IsPublic    bool      `json:"is_public"`
	IconURL     string    `json:"icon_url"`
	CreatedAt   time.Time `json:"created_at"`
	Releases    []Release `json:"releases"`
}

type Release struct {
	ID       int64  `json:"id"`
	Platform string `json:"platform"`
	Filename string `json:"filename"`
	FileSize int64  `json:"file_size"`
}

type CreateInput struct {
	Name        string
	Version     string
	Description string
	IsPublic    bool
}

type UpdateInput struct {
	Name        string
	Version     string
	Description string
	IsPublic    bool
}

// Store is the one seam onto the apps/releases tables and the icons/releases
// directories. Construct it once with the real *sql.DB; tests construct
// their own with a temp DB and temp directories for full isolation.
type Store struct {
	db          *sql.DB
	iconsDir    string
	releasesDir string
}

func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithDirs(db, "icons", "releases")
}

// NewStoreWithDirs constructs a Store rooted at the given directories,
// creating them if they don't already exist — Store owns its directory
// layout, so callers don't separately need to know to create it first.
func NewStoreWithDirs(db *sql.DB, iconsDir, releasesDir string) (*Store, error) {
	if err := os.MkdirAll(iconsDir, 0755); err != nil {
		return nil, fmt.Errorf("apps: create icons dir: %w", err)
	}
	if err := os.MkdirAll(releasesDir, 0755); err != nil {
		return nil, fmt.Errorf("apps: create releases dir: %w", err)
	}
	return &Store{db: db, iconsDir: iconsDir, releasesDir: releasesDir}, nil
}

// TempUploadPath returns a scratch path, inside the same directory tree
// AddRelease manages, that a caller can stream a chunked upload into before
// calling AddRelease to finalize it. Keeping this on Store means the
// directory layout for release files stays known in one place.
func (s *Store) TempUploadPath(uploadID string) string {
	return filepath.Join(s.releasesDir, ".tmp-"+uploadID)
}

// ReleaseFilePath resolves a Release's filename to its on-disk path, for
// callers that need to stream the file out (e.g. serving a download).
func (s *Store) ReleaseFilePath(filename string) string {
	return filepath.Join(s.releasesDir, filename)
}

var allowedIconExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}

func iconURL(iconFilename string) string {
	if iconFilename == "" {
		return ""
	}
	return "/icons/" + iconFilename
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

func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint
}

// Get loads a single App by slug, with its Releases, regardless of
// Visibility — callers decide what to do with a Private App.
func (s *Store) Get(slug string) (App, error) {
	var a App
	var isPublicInt int
	var iconFilename string
	err := s.db.QueryRow(
		"SELECT id, name, slug, description, version, is_public, icon_filename, created_at FROM apps WHERE slug=?", slug,
	).Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.Version, &isPublicInt, &iconFilename, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return App{}, ErrNotFound
	}
	if err != nil {
		return App{}, fmt.Errorf("apps: get %q: %w", slug, err)
	}
	a.IsPublic = isPublicInt == 1
	a.IconURL = iconURL(iconFilename)

	releases, err := s.releasesForApp(a.ID)
	if err != nil {
		return App{}, err
	}
	a.Releases = releases
	return a, nil
}

func (s *Store) releasesForApp(appID int64) ([]Release, error) {
	rows, err := s.db.Query("SELECT id, platform, filename, file_size FROM releases WHERE app_id=? ORDER BY platform", appID)
	if err != nil {
		return nil, fmt.Errorf("apps: load releases for app %d: %w", appID, err)
	}
	defer rows.Close()
	releases := []Release{}
	for rows.Next() {
		var rel Release
		if err := rows.Scan(&rel.ID, &rel.Platform, &rel.Filename, &rel.FileSize); err != nil {
			return nil, fmt.Errorf("apps: scan release: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// List loads every App, with its Releases. onlyPublic restricts the result
// to Apps with Visibility set to Public — callers decide that filter based
// on whether the request is authenticated, not Store.
func (s *Store) List(onlyPublic bool) ([]App, error) {
	query := "SELECT id, name, slug, description, version, is_public, icon_filename, created_at FROM apps ORDER BY created_at DESC"
	if onlyPublic {
		query = "SELECT id, name, slug, description, version, is_public, icon_filename, created_at FROM apps WHERE is_public=1 ORDER BY created_at DESC"
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("apps: list: %w", err)
	}
	defer rows.Close()

	result := []App{}
	for rows.Next() {
		var a App
		var isPublicInt int
		var iconFilename string
		if err := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.Version, &isPublicInt, &iconFilename, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("apps: scan app: %w", err)
		}
		a.IsPublic = isPublicInt == 1
		a.IconURL = iconURL(iconFilename)
		a.Releases = []Release{}
		result = append(result, a)
	}
	rows.Close()

	releasesByApp, err := s.releasesByApp()
	if err != nil {
		return nil, err
	}
	for i := range result {
		if rels, ok := releasesByApp[result[i].ID]; ok {
			result[i].Releases = rels
		}
	}
	return result, nil
}

func (s *Store) releasesByApp() (map[int64][]Release, error) {
	releasesByApp := map[int64][]Release{}
	rows, err := s.db.Query("SELECT id, app_id, platform, filename, file_size FROM releases ORDER BY platform")
	if err != nil {
		return nil, fmt.Errorf("apps: load releases: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rel Release
		var appID int64
		if err := rows.Scan(&rel.ID, &appID, &rel.Platform, &rel.Filename, &rel.FileSize); err != nil {
			return nil, fmt.Errorf("apps: scan release: %w", err)
		}
		releasesByApp[appID] = append(releasesByApp[appID], rel)
	}
	return releasesByApp, nil
}

// Create adds a new App with no Releases yet. Releases are added afterward
// via AddRelease.
func (s *Store) Create(in CreateInput) (App, error) {
	if in.Name == "" || in.Version == "" {
		return App{}, fmt.Errorf("%w: name and version are required", ErrInvalid)
	}

	isPublic := 0
	if in.IsPublic {
		isPublic = 1
	}
	slug := slugify(in.Name)

	res, err := s.db.Exec(
		"INSERT INTO apps (name, slug, description, version, is_public) VALUES (?,?,?,?,?)",
		in.Name, slug, in.Description, in.Version, isPublic,
	)
	if isUniqueConstraint(err) {
		return App{}, ErrSlugTaken
	}
	if err != nil {
		return App{}, fmt.Errorf("apps: create: %w", err)
	}
	id, _ := res.LastInsertId()

	return App{
		ID:          id,
		Name:        in.Name,
		Slug:        slug,
		Description: in.Description,
		Version:     in.Version,
		IsPublic:    in.IsPublic,
		Releases:    []Release{},
	}, nil
}

// Update edits an App's metadata. The Slug is intentionally never changed
// here, even if Name changes — it's part of the public download/detail
// URLs, and silently breaking those on a rename would be worse than letting
// the Slug and displayed Name drift apart.
func (s *Store) Update(id int64, in UpdateInput) error {
	if in.Name == "" || in.Version == "" {
		return fmt.Errorf("%w: name and version are required", ErrInvalid)
	}

	isPublic := 0
	if in.IsPublic {
		isPublic = 1
	}

	res, err := s.db.Exec(
		"UPDATE apps SET name=?, version=?, description=?, is_public=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		in.Name, in.Version, in.Description, isPublic, id,
	)
	if err != nil {
		return fmt.Errorf("apps: update %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetIcon replaces an App's icon image and returns its new IconURL. Only
// raster image formats are accepted — SVG is deliberately excluded since it
// can embed scripts, and these icons get served back out to every visitor.
func (s *Store) SetIcon(appID int64, ext string, r io.Reader) (string, error) {
	ext = strings.ToLower(ext)
	if !allowedIconExt[ext] {
		return "", fmt.Errorf("%w: icon must be a PNG, JPG, or WebP image", ErrInvalid)
	}

	var slug, oldIcon string
	err := s.db.QueryRow("SELECT slug, icon_filename FROM apps WHERE id=?", appID).Scan(&slug, &oldIcon)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("apps: set icon %d: %w", appID, err)
	}

	filename := fmt.Sprintf("%s-icon%s", slug, ext)
	destPath := filepath.Join(s.iconsDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("apps: could not save icon: %w", err)
	}
	defer dest.Close()
	if _, err := io.Copy(dest, r); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("apps: upload was interrupted before it finished: %w", err)
	}

	if _, err := s.db.Exec("UPDATE apps SET icon_filename=? WHERE id=?", filename, appID); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("apps: set icon %d: %w", appID, err)
	}

	// Clean up the previous icon if its filename/extension changed.
	if oldIcon != "" && oldIcon != filename {
		os.Remove(filepath.Join(s.iconsDir, oldIcon))
	}

	return iconURL(filename), nil
}

// AddRelease finalizes a fully-received upload (tempPath) into a Release of
// the given App, naming and moving the file into the releases directory and
// detecting its Platform from originalFilename.
func (s *Store) AddRelease(appID int64, tempPath, originalFilename string, size int64) (Release, error) {
	var slug, version string
	err := s.db.QueryRow("SELECT slug, version FROM apps WHERE id=?", appID).Scan(&slug, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return Release{}, ErrNotFound
	}
	if err != nil {
		return Release{}, fmt.Errorf("apps: add release to app %d: %w", appID, err)
	}

	finalFilename := fmt.Sprintf("%s-%s-%s", slug, version, originalFilename)
	finalPath := filepath.Join(s.releasesDir, finalFilename)
	if err := os.Rename(tempPath, finalPath); err != nil {
		return Release{}, fmt.Errorf("apps: could not finalize upload: %w", err)
	}

	platform := detectPlatform(originalFilename)
	res, err := s.db.Exec(
		"INSERT INTO releases (app_id, platform, filename, file_size) VALUES (?,?,?,?)",
		appID, platform, finalFilename, size,
	)
	if err != nil {
		os.Remove(finalPath)
		return Release{}, fmt.Errorf("apps: add release to app %d: %w", appID, err)
	}
	id, _ := res.LastInsertId()
	return Release{ID: id, Platform: platform, Filename: finalFilename, FileSize: size}, nil
}

// Delete removes an App, its Releases, and their files.
func (s *Store) Delete(id int64) error {
	rows, err := s.db.Query("SELECT filename FROM releases WHERE app_id=?", id)
	if err != nil {
		return fmt.Errorf("apps: delete %d: %w", id, err)
	}
	var filenames []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return fmt.Errorf("apps: delete %d: %w", id, err)
		}
		filenames = append(filenames, f)
	}
	rows.Close()

	var iconFilename string
	s.db.QueryRow("SELECT icon_filename FROM apps WHERE id=?", id).Scan(&iconFilename)

	res, err := s.db.Exec("DELETE FROM apps WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("apps: delete %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.db.Exec("DELETE FROM releases WHERE app_id=?", id)

	for _, f := range filenames {
		os.Remove(filepath.Join(s.releasesDir, f))
	}
	if iconFilename != "" {
		os.Remove(filepath.Join(s.iconsDir, iconFilename))
	}
	return nil
}

// DeleteRelease removes a single platform-specific Release, leaving the
// rest of its App's Releases intact.
func (s *Store) DeleteRelease(id int64) error {
	var filename string
	err := s.db.QueryRow("SELECT filename FROM releases WHERE id=?", id).Scan(&filename)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("apps: delete release %d: %w", id, err)
	}
	if _, err := s.db.Exec("DELETE FROM releases WHERE id=?", id); err != nil {
		return fmt.Errorf("apps: delete release %d: %w", id, err)
	}
	if filename != "" {
		os.Remove(filepath.Join(s.releasesDir, filename))
	}
	return nil
}
