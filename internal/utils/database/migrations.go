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
	&databaseMigration{
		version: 1,
		name:    "clips table",
		migrationScript: `
	CREATE TABLE IF NOT EXISTS clip (
		id INTEGER PRIMARY KEY AUTOINCREMENT
	)
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
