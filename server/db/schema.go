package db

import (
	"database/sql"
	"log"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init(path string) {
	var err error
	DB, err = sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS apps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		description TEXT,
		version TEXT NOT NULL,
		is_public INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS releases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id INTEGER NOT NULL,
		platform TEXT NOT NULL,
		filename TEXT NOT NULL,
		file_size INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(app_id) REFERENCES apps(id)
	);
	`
	_, err = DB.Exec(schema)
	if err != nil {
		log.Fatal("Failed to create schema:", err)
	}

	migrateLegacySingleFileApps()

	// Ensure at least one admin user exists (admin/admin123 — change after first login)
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&count)
	if count == 0 {
		DB.Exec(`INSERT INTO users (username, email, password_hash, role) VALUES ('admin', 'admin@local', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin')`)
		log.Println("Default admin created: username=admin password=password")
	}
}

// migrateLegacySingleFileApps upgrades databases created before apps supported
// multiple per-platform files: the old "apps" table had platform/filename/file_size
// columns directly on it. If those columns are still present, move that data into
// "releases" (one row per app) and drop them from "apps".
func migrateLegacySingleFileApps() {
	rows, err := DB.Query("PRAGMA table_info(apps)")
	if err != nil {
		log.Fatal("Failed to inspect apps schema:", err)
	}
	hasLegacyColumns := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == "platform" {
			hasLegacyColumns = true
		}
	}
	rows.Close()
	if !hasLegacyColumns {
		return
	}

	log.Println("Migrating legacy single-file apps schema...")
	tx, err := DB.Begin()
	if err != nil {
		log.Fatal("Failed to begin migration:", err)
	}

	// The pre-existing "releases" table predates per-platform files and was
	// never written to by any handler, so it's always empty here — safe to
	// replace with the new shape that includes "platform".
	_, err = tx.Exec(`DROP TABLE IF EXISTS releases`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to drop legacy releases table:", err)
	}
	_, err = tx.Exec(`
		CREATE TABLE releases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id INTEGER NOT NULL,
			platform TEXT NOT NULL,
			filename TEXT NOT NULL,
			file_size INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(app_id) REFERENCES apps(id)
		)
	`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to create new releases table:", err)
	}

	_, err = tx.Exec(`
		INSERT INTO releases (app_id, platform, filename, file_size, created_at)
		SELECT id, platform, filename, file_size, created_at FROM apps
	`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to migrate release rows:", err)
	}

	_, err = tx.Exec(`
		CREATE TABLE apps_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			description TEXT,
			version TEXT NOT NULL,
			is_public INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to create apps_new:", err)
	}

	_, err = tx.Exec(`
		INSERT INTO apps_new (id, name, slug, description, version, is_public, created_at, updated_at)
		SELECT id, name, slug, description, version, is_public, created_at, updated_at FROM apps
	`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to copy apps data:", err)
	}

	_, err = tx.Exec(`DROP TABLE apps`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to drop legacy apps table:", err)
	}

	_, err = tx.Exec(`ALTER TABLE apps_new RENAME TO apps`)
	if err != nil {
		tx.Rollback()
		log.Fatal("Failed to rename apps_new:", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatal("Failed to commit migration:", err)
	}
	log.Println("Migration complete.")
}
