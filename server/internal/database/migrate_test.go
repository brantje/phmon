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

func TestObservationSchemaCompatibilityPreservesLegacyLevels(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run migration integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `CREATE SCHEMA navigation_legacy_schema_test;
        SET LOCAL search_path TO navigation_legacy_schema_test;
        CREATE TABLE mob_observations (level SMALLINT, level_source TEXT, level_dataset_id TEXT,
            CONSTRAINT mob_observation_level_provenance CHECK (
                (level IS NULL AND level_source IS NULL AND level_dataset_id IS NULL)
                OR (level IS NOT NULL AND level_source='catalog' AND level_dataset_id IS NOT NULL)));
        INSERT INTO mob_observations VALUES (84,'catalog','gamedata-fixture');
        CREATE TABLE activity_events (region INTEGER CHECK (region BETWEEN 0 AND 65535));`)
	if err != nil {
		t.Fatal(err)
	}
	body, err := migrationFiles.ReadFile("migrations/000023_observation_schema_compatibility.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(body)); err != nil {
		t.Fatal(err)
	}
	var level, legacyLevel int
	var source, legacySource, dataset string
	err = tx.QueryRow(ctx, `SELECT resolved_level,level_source,level,legacy_level_source,level_dataset_id FROM mob_observations`).Scan(&level, &source, &legacyLevel, &legacySource, &dataset)
	if err != nil {
		t.Fatal(err)
	}
	if level != 84 || legacyLevel != 84 || source != "catalog" || legacySource != source || dataset != "gamedata-fixture" {
		t.Fatal("legacy evidence was not preserved")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mob_observations(resolved_level,level_source) VALUES(85,'runtime'); INSERT INTO activity_events(region) VALUES(-32767);`); err != nil {
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
