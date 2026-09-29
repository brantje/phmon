package database

import (
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEmbeddedMigrationVersionsAreUnique(t *testing.T) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMigrationVersions(entries); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMigrationVersionsRejectsDuplicates(t *testing.T) {
	files := fstest.MapFS{
		"000015_level_up.sql":       &fstest.MapFile{},
		"000015_signed_regions.sql": &fstest.MapFile{},
	}
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMigrationVersions(entries); err == nil || !strings.Contains(err.Error(), "duplicate migration version 15") {
		t.Fatalf("duplicate migration version error = %v", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run migration integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("cannot initialize test database pool")
	}
	defer pool.Close()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.agents') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("agents table was not created")
	}
}
