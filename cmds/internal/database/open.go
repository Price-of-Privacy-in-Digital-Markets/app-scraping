package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
)

func OpenMemory(store uint8, version uint8) (*sql.DB, error) {
	dsn := "file:test.db?mode=memory&_foreign_keys=true"
	db, err := sql.Open("sqlite3_custom", dsn)
	if err != nil {
		return nil, err
	}

	userVersion := EncodeUserVersion(store, version)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", userVersion)); err != nil {
		return nil, err
	}

	if err := configureDatabase(db, store, version); err != nil {
		return nil, err
	}

	return db, nil
}

func OpenOrCreate(databasePath string, store uint8, version uint8) (db *sql.DB, created bool, err error) {
	absPath, err := filepath.Abs(databasePath)
	if err != nil {
		return
	}

	dsn := fmt.Sprintf("file:%s?mode=rw&_foreign_keys=true&_journal_mode=WAL&_synchronous=NORMAL", absPath)
	db, err = sql.Open("sqlite3_custom", dsn)
	if err != nil || db.Ping() != nil {
		dsn := fmt.Sprintf("file:%s?mode=rwc&_foreign_keys=true&_journal_mode=WAL&_synchronous=NORMAL", absPath)
		db, err = sql.Open("sqlite3_custom", dsn)
		if err != nil {
			return
		}

		if err = db.Ping(); err != nil {
			return
		}

		created = true

		userVersion := EncodeUserVersion(store, version)
		if _, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", userVersion)); err != nil {
			return
		}
	}

	err = configureDatabase(db, store, version)
	return
}

func configureDatabase(db *sql.DB, store uint8, version uint8) error {
	// Disable connection pooling
	db.SetMaxOpenConns(1)

	// Check that the user version is as expected
	var userVersion int32
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		return err
	}

	dbStore, dbVersion, err := DecodeUserVersion(userVersion)
	if err != nil {
		return err
	}

	if store != dbStore {
		return fmt.Errorf("invalid database: store is %d but expected %d", dbStore, store)
	}

	if version != dbVersion {
		return fmt.Errorf("invalid database: version is %d but expected %d", dbVersion, version)
	}

	return nil
}
