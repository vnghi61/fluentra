// Command seed loads the catalogue (courses, words, mock tests) and, for local
// development, two demo accounts.
//
// `-reference` loads the catalogue only, attributed to the first admin, and is
// the one mode allowed in production: register, promote the account with
// db/seeds/rbac.sql, then seed. Demo accounts are never created there.
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
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/dbtarget"
	authdomain "github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	"github.com/fluentra/fluentra/internal/modules/user"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/config"
)

// demoPassword is the one CONTRIBUTING.md prints. It exists only here and is
// refused in production; nothing else in this repository ships a password.
const demoPassword = "Password123!demo"

type seedConfig struct {
	App struct {
		Environment string `koanf:"env"`
	} `koanf:"app"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	Redis struct {
		URL string `koanf:"url"`
	} `koanf:"redis"`
	// Storage is where the dashboard slides are uploaded: the same S3_* the
	// API serves banner images from.
	Storage struct {
		Endpoint      string `koanf:"endpoint"`
		AccessKey     string `koanf:"access_key"`
		SecretKey     string `koanf:"secret_key"`
		Region        string `koanf:"region"`
		UseSSL        bool   `koanf:"use_ssl"`
		UsePostPolicy bool   `koanf:"use_post_policy"`
	} `koanf:"s3"`
}

type demoAccount struct {
	email       string
	displayName string
	admin       bool
	role        string
}

// The accounts CONTRIBUTING.md names. Changing one here means changing it
// there. They use the reserved example.test domain, so no real inbox (and no
// Google account) can ever claim them.
//
// The admin holds every permission (core.role_permissions grants it the whole
// set), so one back-office account covers the review queue, the question bank
// and content authoring. The learner is separate because the learner-facing
// flows — uploading a resource, sitting a lesson, the daily set — have to be
// exercised by an account that holds no permissions at all, or a paywall or a
// redaction that only bites a learner would never be seen failing.
var demoAccounts = []demoAccount{
	{email: "learner@example.test", displayName: "Demo Learner"},
	// Not "Demo Admin": BR-USER-02 rejects any display name containing "admin".
	{email: "admin@example.test", displayName: "Demo Editor", admin: true},
}

// includeFoundationFixtures is set from -foundation and read by run. Default
// on: `make seed` is the one command that has to produce a complete catalogue
// (WO 22 Stage G).
var includeFoundationFixtures = true

// includeExamFixtures is set from -exams and read by run. Default on: a
// database with no exam content is a valid state the loader reports, not fails.
var includeExamFixtures = true

// foundationCoursesOnly repairs lesson bindings without refreshing demo accounts
// or replaying content fixtures over already published content.
var foundationCoursesOnly bool

// referenceOnly is set from -reference: the catalogue without demo accounts.
var referenceOnly bool

// adminEmail is set from -admin-email: the registered account -reference
// promotes before it looks for an admin to own the catalogue.
var adminEmail string

// allowRemoteDB is set from -allow-remote-db: see dbtarget.Check.
var allowRemoteDB bool

// includeBanners is set from -banners.
var includeBanners = true

func main() {
	audio := flag.Bool("audio", false,
		"backfill recorded pronunciation onto flashcards that have none, then exit")
	foundation := flag.Bool("foundation", true,
		"load the frozen Foundation fixtures from db/fixtures/foundation (no model call)")
	exams := flag.Bool("exams", true,
		"load the frozen exam fixtures from db/fixtures/exams (no model call)")

	flag.BoolVar(&referenceOnly, "reference", false,
		"load the catalogue only, owned by the first admin; no demo accounts (allowed in production)")
	flag.BoolVar(&foundationCoursesOnly, "foundation-courses-only", false,
		"sync Foundation lessons from existing published content, without changing accounts or fixtures")
	flag.BoolVar(&includeBanners, "banners", true,
		"upload the dashboard slides in db/fixtures/banners to S3_* storage and create their banners")
	flag.IntVar(&seedConcurrency, "concurrency", defaultSeedConcurrency,
		"how many workers write at once; 1 seeds one row at a time")
	flag.StringVar(&adminEmail, "admin-email", "",
		"with -reference, grant the admin role to this already registered account first (bootstraps the first admin)")
	flag.BoolVar(&allowRemoteDB, "allow-remote-db", false,
		"seed a remote database even though APP_ENV does not say production or staging")
	flag.Parse()
	includeFoundationFixtures = *foundation
	includeExamFixtures = *exams
	if *audio {
		if err := runAudioBackfill(context.Background(), os.Stdout); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if err := run(context.Background(), os.Stdout); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// seedDemoAccounts creates or refreshes the demo logins and returns the admin's
// id, which the authored content is attributed to.
func seedDemoAccounts(
	ctx context.Context,
	pool *pgxpool.Pool,
	creator usercontract.Creator,
	roles *rbac.Module,
	hash string,
	out io.Writer,
) (uuid.UUID, error) {
	var adminID uuid.UUID
	for _, account := range demoAccounts {
		id, created, err := ensureAccount(ctx, pool, creator, account)
		if err != nil {
			return uuid.Nil, fmt.Errorf("seed %s: %w", account.email, err)
		}
		if err := ensureCredential(ctx, pool, id, hash); err != nil {
			return uuid.Nil, fmt.Errorf("seed credential for %s: %w", account.email, err)
		}
		if err := ensureVerified(ctx, pool, id); err != nil {
			return uuid.Nil, fmt.Errorf("verify %s: %w", account.email, err)
		}
		// Also for accounts that already existed. user.New grants the role while
		// creating an account, and a re-run against a database seeded before this
		// existed would otherwise leave the demo learner exactly as it was:
		// signed in, and refused the catalogue. The grant is idempotent.
		if err := roles.GrantBaselineRole(ctx, id); err != nil {
			return uuid.Nil, fmt.Errorf("grant baseline role to %s: %w", account.email, err)
		}
		if account.admin {
			adminID = id
			if err := ensureAdmin(ctx, pool, id); err != nil {
				return uuid.Nil, fmt.Errorf("grant admin to %s: %w", account.email, err)
			}
		}
		if account.role != "" {
			if err := ensureRole(ctx, pool, id, account.role); err != nil {
				return uuid.Nil, fmt.Errorf("grant %s to %s: %w", account.role, account.email, err)
			}
		}

		state := "already present, refreshed"
		if created {
			state = "created"
		}
		_, _ = fmt.Fprintf(out, "%-24s %s\n", account.email, state)
	}
	return adminID, nil
}

func run(ctx context.Context, out io.Writer) error {
	var cfg seedConfig
	// `app.env` has to be declared, not merely read into the struct. config.Load
	// only accepts environment variables whose section appears in Defaults,
	// Required or EnvSections — so without the default below, APP_ENV is
	// dropped, `cfg.App.Environment` stays empty, and the production guard can
	// never fire. TestRun_RefusesProduction is what caught that.
	if err := config.Load(ctx, config.Options{
		Defaults: map[string]any{
			"app.env": "local", "redis.url": "",
			// .env.example's local MinIO; production passes the R2 values.
			"s3.endpoint": "localhost:9000", "s3.access_key": "minioadmin", "s3.secret_key": "minioadmin",
			"s3.region": "us-east-1", "s3.use_ssl": false, "s3.use_post_policy": true,
		},
		Required: []config.RequiredKey{{
			Name:       "db.dsn",
			DocSection: ".env.example",
		}},
	}, &cfg); err != nil {
		return fmt.Errorf("load seed configuration: %w", err)
	}

	// A guard, not a convention. Seeding writes accounts with a password that
	// is printed in a public guide; the one environment where that must never
	// happen is worth refusing by name rather than by care.
	if err := refuseProduction(cfg.App.Environment); err != nil {
		return err
	}
	// And the name has to match where the DSN goes: APP_ENV=local with a
	// production DSN would otherwise sail past the guard above.
	if err := dbtarget.Check(cfg.App.Environment, cfg.Database.DSN, allowRemoteDB); err != nil {
		return err
	}
	if adminEmail != "" && !referenceOnly {
		return errors.New("-admin-email bootstraps the owner of a -reference seed; use it with -reference")
	}

	pool, err := newSeedPool(ctx, cfg.Database.DSN, seedConcurrency)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The demo learner needs the baseline role for the same reason a registered
	// one does: `user` holds content.read.published, and without it the account
	// the getting-started guide hands out is refused the very catalogue this
	// seed then authors. user.New grants it as part of creating the account.
	roles := rbac.New(rbac.Deps{Pool: pool})
	if foundationCoursesOnly {
		return seedConfiguredFoundationCourses(ctx, pool, uuid.Nil, cfg, out)
	}
	adminID, err := contentOwner(ctx, pool, roles, out)
	if err != nil {
		return err
	}

	// The content is owned by the admin account, so a run that produced no admin
	// has nothing to attribute it to. That is a broken seed, not a partial one:
	// silently skipping left a database with logins and no curriculum, which
	// looks like a working seed until someone signs in.
	if adminID == uuid.Nil {
		return fmt.Errorf("no admin account was seeded, so content cannot be authored")
	}
	if err := seedCatalogue(ctx, pool, adminID, cfg, out); err != nil {
		return err
	}

	if !referenceOnly {
		_, _ = fmt.Fprintf(out, "\npassword for all: %s\n", demoPassword)
	}
	return nil
}

// seedCatalogue loads everything the admin account owns: the authored
// courses and words, the Foundation content and courses, the mock tests and
// the dashboard slides, in that order.
func seedCatalogue(ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, cfg seedConfig, out io.Writer) error {
	if err := seedAuthoredContent(ctx, pool, adminID, out); err != nil {
		return err
	}

	// The frozen Foundation content (WO 22 Stage G): loaded through content's
	// own state machine, with the approval it already had.
	if includeFoundationFixtures {
		if err := seedFoundationFixtures(ctx, pool, adminID, defaultFoundationFixtureDir, out); err != nil {
			return fmt.Errorf("seed foundation fixtures: %w", err)
		}
	}

	// The thirteen Foundation courses (WO 22 Stage H), built from the course
	// map through lesson's Author, after the fixtures: a lesson's activities are
	// its node's published content, so building the courses first gave a fresh
	// database thirteen courses of empty lessons. This also archives the five
	// Phase 2 courses seedAuthoredContent authored, so the catalogue shows
	// thirteen.
	if err := seedConfiguredFoundationCourses(ctx, pool, adminID, cfg, out); err != nil {
		return fmt.Errorf("seed Foundation courses: %w", err)
	}

	// The frozen exam content (WO 22 Stage N): loaded through content's own
	// state machine, put in the bank, then composed into numbered fixed tests.
	if includeExamFixtures {
		if err := seedExamFixtures(ctx, pool, adminID, defaultExamFixtureDir, out); err != nil {
			return fmt.Errorf("seed exam fixtures: %w", err)
		}
	}

	// The dashboard's promotional slides (tools/banners), last: they are the
	// one step that needs object storage as well as the database.
	if includeBanners {
		store, err := newSeedStore(cfg)
		if err != nil {
			return err
		}
		if err := seedBannerSlides(ctx, pool, store, adminID, defaultBannerFixtureDir, out); err != nil {
			return fmt.Errorf("seed banners: %w", err)
		}
	}
	return nil
}

// refuseProduction allows only -reference in production: every other mode
// writes the demo accounts or rebuilds courses around them.
func refuseProduction(environment string) error {
	if environment == "production" && (!referenceOnly || foundationCoursesOnly) {
		return errors.New("refusing to seed demo accounts: APP_ENV is production (use -reference)")
	}
	return nil
}

// contentOwner returns the account the catalogue is attributed to: the demo
// admin it seeds, or with -reference an admin that already exists.
func contentOwner(ctx context.Context, pool *pgxpool.Pool, roles *rbac.Module, out io.Writer) (uuid.UUID, error) {
	if referenceOnly {
		if adminEmail != "" {
			if err := promoteRegisteredAdmin(ctx, pool, adminEmail, out); err != nil {
				return uuid.Nil, err
			}
		}
		id, err := roles.RoleMembers().FirstHolderOf(ctx, "admin")
		if err != nil {
			return uuid.Nil, fmt.Errorf("find an admin to own the catalogue: %w", err)
		}
		if id == uuid.Nil {
			return uuid.Nil, errors.New("no admin yet: register an account and promote it (db/seeds/rbac.sql) first")
		}
		return id, nil
	}
	module := user.New(user.Deps{Pool: pool, Roles: roles})
	hash, err := authdomain.NewHasher(authdomain.DefaultHashParams()).Hash(demoPassword)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash demo password: %w", err)
	}
	return seedDemoAccounts(ctx, pool, module.Creator(), roles, hash, out)
}

// seedAuthoredContent writes everything that is authored rather than earned:
// the curriculum, and the badge and quest catalogues.
//
// Split out of run() to keep that function under the cyclomatic limit, and
// because the two calls belong together: both are idempotent upserts of content
// an operator maintains, and neither writes a single row of learner state.
func seedAuthoredContent(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, out io.Writer,
) error {
	if err := seedContentAndCurriculum(ctx, pool, adminID, out); err != nil {
		return fmt.Errorf("seed content & curriculum: %w", err)
	}
	if err := seedGamification(ctx, pool, out); err != nil {
		return fmt.Errorf("seed gamification catalogue: %w", err)
	}
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

// ensureRole grants a role by name to a user account.
// granted_by stays NULL because the system made this grant, not a person.
func ensureRole(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, roleName string) error {
	const grant = `
		INSERT INTO core.user_roles (user_id, role_id, granted_by)
		SELECT $1, r.id, NULL FROM core.roles r WHERE r.name = $2
		ON CONFLICT (user_id, role_id) DO NOTHING`
	_, err := pool.Exec(ctx, grant, userID, roleName)
	return err
}

// promoteRegisteredAdmin grants the admin role to an account somebody already
// registered and verified, the same grant db/seeds/rbac.sql makes, so a fresh
// production database gets its first admin without a psql session. It never
// creates an account: the person signs up through the app like anyone else.
func promoteRegisteredAdmin(ctx context.Context, pool *pgxpool.Pool, email string, out io.Writer) error {
	var userID uuid.UUID
	var verified bool
	var status string
	err := pool.QueryRow(ctx,
		`SELECT id, email_verified_at IS NOT NULL, status::text FROM core.users WHERE email = $1`,
		strings.TrimSpace(email)).Scan(&userID, &verified, &status)
	if pgxIsNoRows(err) {
		return fmt.Errorf("no account for %s: register it in the app first, then re-run", email)
	}
	if err != nil {
		return fmt.Errorf("look up %s: %w", email, err)
	}
	if status != "active" {
		return fmt.Errorf("%s is %s, not active: it cannot own the catalogue", email, status)
	}
	if !verified {
		return fmt.Errorf("%s has not verified its email yet: finish sign-up, then re-run", email)
	}
	if err := ensureAdmin(ctx, pool, userID); err != nil {
		return fmt.Errorf("grant admin to %s: %w", email, err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ admin role granted to %s\n", email)
	return nil
}

// ensureAdmin grants the admin role, the same insert db/seeds/rbac.sql makes.
func ensureAdmin(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) error {
	return ensureRole(ctx, pool, userID, "admin")
}
