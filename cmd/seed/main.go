// Command seed loads the development dataset the getting-started guide promises.
//
// It was `func main() {}` — a documented step (`make seed`, and the two demo
// accounts named in docs/development/getting-started.md §4) that did nothing at
// all. A new contributor followed the guide, saw no error, and then could not
// sign in with the credentials the guide had just given them.
//
// It writes through the same paths the application uses: the user module's
// Creator contract for the three-row account transaction, the auth domain's
// Argon2id hasher for the password, and db/seeds/rbac.sql's own grant for the
// admin role. Nothing here reaches into another module's internals, and nothing
// invents a column the API does not already write.
//
// Idempotent: run it as often as you like. Never run it against production —
// it refuses when APP_ENV says so.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authdomain "github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/user"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/config"
)

// demoPassword is the one the getting-started guide prints. It exists only
// here; nothing else in this repository ships a default password.
const demoPassword = "Password123!demo"

type seedConfig struct {
	App struct {
		Environment string `koanf:"env"`
	} `koanf:"app"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
}

type demoAccount struct {
	email       string
	displayName string
	admin       bool
}

// The two accounts docs/development/getting-started.md §4 names. Changing one
// here means changing it there.
var demoAccounts = []demoAccount{
	{email: "learner@fluentra.dev", displayName: "Demo Learner"},
	{email: "admin@fluentra.dev", displayName: "Demo Operator", admin: true},
}

func main() {
	if err := run(context.Background(), os.Stdout); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out io.Writer) error {
	var cfg seedConfig
	// `app.env` has to be declared, not merely read into the struct. config.Load
	// only accepts environment variables whose section appears in Defaults,
	// Required or EnvSections — so without the default below, APP_ENV is
	// dropped, `cfg.App.Environment` stays empty, and the production guard can
	// never fire. TestRun_RefusesProduction is what caught that.
	if err := config.Load(ctx, config.Options{
		Defaults: map[string]any{"app.env": "local"},
		Required: []config.RequiredKey{{
			Name:       "db.dsn",
			DocSection: "docs/deployment/configuration.md#database",
		}},
	}, &cfg); err != nil {
		return fmt.Errorf("load seed configuration: %w", err)
	}

	// A guard, not a convention. Seeding writes accounts with a password that
	// is printed in a public guide; the one environment where that must never
	// happen is worth refusing by name rather than by care.
	if cfg.App.Environment == "production" {
		return errors.New("refusing to seed: APP_ENV is production")
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	module := user.New(user.Deps{Pool: pool})
	hasher := authdomain.NewHasher(authdomain.DefaultHashParams())

	hash, err := hasher.Hash(demoPassword)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	for _, account := range demoAccounts {
		id, created, err := ensureAccount(ctx, pool, module.Creator(), account)
		if err != nil {
			return fmt.Errorf("seed %s: %w", account.email, err)
		}
		if err := ensureCredential(ctx, pool, id, hash); err != nil {
			return fmt.Errorf("seed credential for %s: %w", account.email, err)
		}
		if err := ensureVerified(ctx, pool, id); err != nil {
			return fmt.Errorf("verify %s: %w", account.email, err)
		}
		if account.admin {
			if err := ensureAdmin(ctx, pool, id); err != nil {
				return fmt.Errorf("grant admin to %s: %w", account.email, err)
			}
		}

		state := "already present, refreshed"
		if created {
			state = "created"
		}
		fmt.Fprintf(out, "%-24s %s\n", account.email, state)
	}

	fmt.Fprintf(out, "\npassword for both: %s\n", demoPassword)
	return nil
}

// ensureAccount creates the account if the address is free, and otherwise
// returns the existing id. The lookup is by address because that is the key a
// re-run has to match on.
func ensureAccount(
	ctx context.Context, pool *pgxpool.Pool, creator usercontract.Creator, account demoAccount,
) (uuid.UUID, bool, error) {
	var existing uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM core.users WHERE email = $1", account.email).
		Scan(&existing)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, fmt.Errorf("look up account: %w", err)
	}

	// The module's own transaction, so the user, profile and preference rows
	// are written the way registration writes them.
	id, err := creator.CreateUser(ctx, usercontract.NewUser{
		Email:       account.email,
		DisplayName: account.displayName,
		Locale:      "en",
		Timezone:    "Asia/Ho_Chi_Minh",
	})
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// ensureCredential sets the password, replacing whatever was there. Re-running
// the seed after changing demoPassword should leave the account usable with the
// password the guide currently prints.
func ensureCredential(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, hash string) error {
	const upsert = `
		INSERT INTO core.credentials (user_id, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET password_hash = EXCLUDED.password_hash, updated_at = now()`
	_, err := pool.Exec(ctx, upsert, userID, hash)
	return err
}

// ensureVerified marks the address verified. A seeded account that still had to
// go through the OTP flow would need an inbox, which defeats the point of a
// dataset meant to make the app usable immediately.
func ensureVerified(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) error {
	const verify = `
		UPDATE core.users
		SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
		WHERE id = $1`
	_, err := pool.Exec(ctx, verify, userID)
	return err
}

// ensureAdmin grants the admin role, the same insert db/seeds/rbac.sql makes.
// granted_by stays NULL because the system made this grant, not a person.
func ensureAdmin(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) error {
	const grant = `
		INSERT INTO core.user_roles (user_id, role_id, granted_by)
		SELECT $1, r.id, NULL FROM core.roles r WHERE r.name = 'admin'
		ON CONFLICT (user_id, role_id) DO NOTHING`
	_, err := pool.Exec(ctx, grant, userID)
	return err
}
