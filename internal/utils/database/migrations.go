package database

import (
	"database/sql"

	"github.com/nantipov/longscreen/internal/utils"
)

type databaseMigration struct {
	version         uint16
	name            string
	migrationScript string
}

var migrations []*databaseMigration = []*databaseMigration{

	/////////////////////////////////////////////////
	&databaseMigration{
		version: 1,
		name:    "base tables",
		migrationScript: `
	CREATE TABLE clip (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		clip_type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'CREATED'
	);
	
	CREATE TABLE video_track (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		clip_id INTEGER NOT NULL,
		width INTEGER NOT NULL,
		height INTEGER NOT NULL
	);
	
	CREATE TABLE audio_track (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		clip_id INTEGER NOT NULL,
		ts0 INTEGER,
		ts1 INTEGER,
		seq INTEGER
	);
	
	CREATE TABLE frame (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		video_track_id INTEGER NOT NULL,
		x_mouse INTEGER,
		y_mouse INTEGER,
		fps INTEGER,
		ts INTEGER,
		seq INTEGER,
		status TEXT NOT NULL DEFAULT 'CREATED'
	);
	
	CREATE UNIQUE INDEX idx_uniq_frame_track_seq ON frame (video_track_id, seq);
		`,
	},
}

func ApplyMigrations(db *sql.DB) {
	applyVersionsTable(db)
	row := db.QueryRow("SELECT coalesce(max(version), 0) max_version FROM migration")
	var maxVersion uint16
	err := row.Scan(&maxVersion)
	if err != nil && err == sql.ErrNoRows {
		maxVersion = 0
	} else {
		utils.HandleError(err)
	}
	for _, migration := range migrations {
		if migration.version > maxVersion {
			applyMigration(db, migration)
		}
	}
}

func applyMigration(db *sql.DB, migration *databaseMigration) {
	versionEntrySql := `INSERT INTO migration (version, name) VALUES (?, ?)`
	tx, err := db.Begin()

	versionEntryStmt, err := tx.Prepare(versionEntrySql)
	utils.HandleError(err)
	defer versionEntryStmt.Close()
	versionEntryStmt.Exec(migration.version, migration.name)

	_, err = tx.Exec(migration.migrationScript)
	utils.HandleError(err)

	tx.Commit()
}

func applyVersionsTable(db *sql.DB) {
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS migration (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		panic(err)
	}
}
