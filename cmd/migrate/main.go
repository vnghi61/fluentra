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
		return errors.New("usage: migrate <up|down|status>")
	}
	command := args[0]
	if command != "up" && command != "down" && command != "status" {
		return fmt.Errorf("unknown migration command %q; expected up, down, or status", command)
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
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// The first migration creates this role. Subsequent commands run as the
	// dedicated migration owner, while the runtime application role remains DDL-free.
	if err := setMigrationRole(ctx, db); err != nil {
		return err
	}

	sources, err := migrationFS()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	defer func() { _ = provider.Close() }()

	switch command {
	case "up":
		return runUp(ctx, provider, db, out)
	case "down":
		return runDown(ctx, provider, out)
	default:
		return runStatus(ctx, provider, out)
	}
}

// runUp applies pending migrations one at a time, re-assuming the migration role
// between each. The bootstrap migration is what creates fluentra_migrator, so on
// a fresh database the role cannot be assumed until after it has run. Applying
// the whole set in one call would leave the bootstrap's tables owned by the
// migrator and everything after it owned by the connecting user — and
// `migrate down` would then fail on tables it does not own.
func runUp(ctx context.Context, provider *goose.Provider, db *sql.DB, out io.Writer) error {
	applied := 0
	for {
		if err := setMigrationRole(ctx, db); err != nil {
			return err
		}
		result, err := provider.UpByOne(ctx)
		if errors.Is(err, goose.ErrNoNextVersion) {
			break
		}
		if err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
		if result == nil {
			break
		}
		applied++
	}
	_, _ = fmt.Fprintf(out, "applied %d migration(s)\n", applied)
	return nil
}

// runDown rolls back the most recently applied migration.
func runDown(ctx context.Context, provider *goose.Provider, out io.Writer) error {
	result, err := provider.Down(ctx)
	if err != nil {
		return fmt.Errorf("roll back migration: %w", err)
	}
	if result == nil {
		_, _ = fmt.Fprintln(out, "no migration to roll back")
		return nil
	}
	_, _ = fmt.Fprintf(out, "rolled back migration %d\n", result.Source.Version)
	return nil
}

// runStatus reports the applied/pending state of every known migration.
func runStatus(ctx context.Context, provider *goose.Provider, out io.Writer) error {
	statuses, err := provider.Status(ctx)
	if err != nil {
		return fmt.Errorf("get migration status: %w", err)
	}
	for _, status := range statuses {
		_, _ = fmt.Fprintf(out, "%s %d %s\n", status.State, status.Source.Version, status.Source.Path)
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

// migrationFS flattens the embedded per-module directories into one virtual
// directory. Goose then orders the globally unique Unix timestamp prefixes
// across modules.
func migrationFS() (fs.FS, error) { return migrations.Flattened() }
