package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// migrate aplica, em ordem, os arquivos migrations/NNN_*.sql ainda não aplicados.
// A versão atual fica no PRAGMA user_version do próprio banco.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	files, err := fs.Glob(embedded, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for i, f := range files {
		n := i + 1
		if n <= version {
			continue
		}
		b, err := fs.ReadFile(embedded, f)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(b)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", f, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", n)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
