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
		name:    "clips table",
		migrationScript: `
	CREATE TABLE IF NOT EXISTS clip (
		id INTEGER PRIMARY KEY AUTOINCREMENT
	)
		`,
	},

	/////////////////////////////////////////////////
	&databaseMigration{
		version: 2,
		name:    "frames table; exporter",
		migrationScript: `
	CREATE TABLE IF NOT EXISTS frame (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		clip_id INTEGER NOT NULL,
		x_mouse INTEGER,
		y_mouse INTEGER,
		ts INTEGER,
		filename TEXT
	);
	
	ALTER TABLE clip ADD COLUMN is_stopped SMALLINT NOT NULL DEFAULT 0;
	ALTER TABLE clip ADD COLUMN is_exported SMALLINT NOT NULL DEFAULT 0;
	ALTER TABLE clip ADD COLUMN clip_type TEXT NOT NULL DEFAULT 'UNKNOWN';
	ALTER TABLE clip ADD COLUMN path TEXT NOT NULL DEFAULT 'clip';
	
	/* mark existing clips as stopped */
	UPDATE clip SET is_stopped = 1;
	
	CREATE TABLE IF NOT EXISTS audio_track (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		clip_id INTEGER NOT NULL,
		start_ts INTEGER,
		end_ts INTEGER,
		filename TEXT
	);
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
