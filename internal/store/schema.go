package store

import (
	"database/sql"
	"fmt"

	"github.com/charmbracelet/log"
)

const currentSchemaVersion = 1

// Schema definitions
const schemaVersionTable = `
CREATE TABLE IF NOT EXISTS schema_version (
	version INTEGER PRIMARY KEY
);
`

const storesTable = `
CREATE TABLE IF NOT EXISTS stores (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	root_path TEXT NOT NULL,
	embedding_provider TEXT NOT NULL,
	embedding_model TEXT NOT NULL,
	embedding_dimensions INTEGER NOT NULL,
	created_at TEXT DEFAULT (datetime('now')),
	updated_at TEXT DEFAULT (datetime('now'))
);
`

const filesTable = `
CREATE TABLE IF NOT EXISTS files (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	store_id INTEGER NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
	external_id TEXT NOT NULL,
	path TEXT NOT NULL,
	relative_path TEXT NOT NULL,
	hash TEXT NOT NULL,
	file_size INTEGER NOT NULL,
	indexed_at TEXT DEFAULT (datetime('now')),
	UNIQUE(store_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_files_store_id ON files(store_id);
CREATE INDEX IF NOT EXISTS idx_files_hash ON files(store_id, hash);
`

const chunksTable = `
CREATE TABLE IF NOT EXISTS chunks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
	chunk_index INTEGER NOT NULL,
	content TEXT NOT NULL,
	start_line INTEGER NOT NULL,
	end_line INTEGER NOT NULL,
	UNIQUE(file_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_chunks_file_id ON chunks(file_id);
`

// createVectorTable creates the sqlite-vec virtual table for the given dimensions.
func createVectorTable(db *sql.DB, dimensions int) error {
	query := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
			chunk_id INTEGER PRIMARY KEY,
			embedding float[%d] distance_metric=cosine
		);
	`, dimensions)

	_, err := db.Exec(query)
	return err
}

// initSchema initializes the database schema.
func initSchema(db *sql.DB) error {
	// Create schema version table
	if _, err := db.Exec(schemaVersionTable); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Check current version
	var version int
	err := db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		version = 0
	} else if err != nil {
		return fmt.Errorf("failed to check schema version: %w", err)
	}

	if version >= currentSchemaVersion {
		log.Debug("Schema is up to date", "version", version)
		return nil
	}

	log.Debug("Migrating schema", "from", version, "to", currentSchemaVersion)

	// Apply migrations
	if version < 1 {
		if err := migrateV1(db); err != nil {
			return fmt.Errorf("failed to migrate to v1: %w", err)
		}
	}

	return nil
}

// migrateV1 creates the initial schema.
func migrateV1(db *sql.DB) error {
	log.Debug("Applying migration v1")

	// Create tables
	tables := []string{storesTable, filesTable, chunksTable}
	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Note: Vector table is created per-store with specific dimensions
	// We'll create it when the first store is created

	// Update schema version
	if _, err := db.Exec("INSERT OR REPLACE INTO schema_version (version) VALUES (?)", 1); err != nil {
		return fmt.Errorf("failed to update schema version: %w", err)
	}

	return nil
}

// ensureVectorTable ensures the vector table exists with the correct dimensions.
// If dimensions change, we need to recreate the table.
func ensureVectorTable(db *sql.DB, dimensions int) error {
	// Check if vector table exists
	var tableName string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='chunk_vectors'
	`).Scan(&tableName)

	if err == sql.ErrNoRows {
		// Table doesn't exist, create it
		log.Debug("Creating vector table", "dimensions", dimensions)
		return createVectorTable(db, dimensions)
	} else if err != nil {
		return fmt.Errorf("failed to check vector table: %w", err)
	}

	// Table exists - for now we assume dimensions match
	// In production, we might want to verify and handle dimension changes
	return nil
}
