package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationRole = "fluentra_migrator"

type databaseConfig struct {
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: migrate <up|down|status>")
	}
	if args[0] != "up" && args[0] != "down" && args[0] != "status" {
		return fmt.Errorf("unknown migration command %q; expected up, down, or status", args[0])
	}

	var cfg databaseConfig
	if err := config.Load(ctx, config.Options{Required: []config.RequiredKey{{
		Name:       "db.dsn",
		DocSection: "docs/deployment/configuration.md#database",
	}}}, &cfg); err != nil {
		return fmt.Errorf("load migration configuration: %w", err)
	}

	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// The first migration creates this role. Subsequent commands run as the
	// dedicated migration owner, while the runtime application role remains DDL-free.
	if err := setMigrationRole(ctx, db); err != nil {
		return err
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS())
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	defer provider.Close()

	switch args[0] {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
		fmt.Fprintf(out, "applied %d migration(s)\n", len(results))
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			return fmt.Errorf("roll back migration: %w", err)
		}
		if result == nil {
			fmt.Fprintln(out, "no migration to roll back")
		} else {
			fmt.Fprintf(out, "rolled back migration %d\n", result.Source.Version)
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("get migration status: %w", err)
		}
		for _, status := range statuses {
			fmt.Fprintf(out, "%s %d %s\n", status.State, status.Source.Version, status.Source.Path)
		}
	}
	return nil
}

func setMigrationRole(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "SET ROLE "+migrationRole); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22023" {
			return nil // Bootstrap has not created the role yet.
		}
		return fmt.Errorf("assume migration role: %w", err)
	}
	return nil
}

// migrationFS flattens per-module directories into one virtual directory.
// Goose then orders the globally unique Unix timestamp prefixes across modules.
func migrationFS() fs.FS {
	files := make(fstest.MapFS)
	if err := fs.WalkDir(migrations.Files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path.Ext(name) != ".sql" {
			return nil
		}
		content, err := migrations.Files.ReadFile(name)
		if err != nil {
			return err
		}
		base := path.Base(name)
		key := strings.TrimSuffix(base, path.Ext(base)) + "_" + strings.ReplaceAll(path.Dir(name), "/", "_") + path.Ext(base)
		if _, exists := files[key]; exists {
			return fmt.Errorf("duplicate migration filename %q", key)
		}
		files[key] = &fstest.MapFile{Data: content, Mode: 0o444}
		return nil
	}); err != nil {
		panic(fmt.Sprintf("load embedded migrations: %v", err))
	}
	return sortedMapFS{MapFS: files}
}

// sortedMapFS makes ordering deterministic even for callers that inspect it directly.
type sortedMapFS struct{ fstest.MapFS }

func (s sortedMapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := s.MapFS.ReadDir(name)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}
