package main

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/audit"
	"github.com/fluentra/fluentra/internal/modules/auth"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/user"
	"github.com/fluentra/fluentra/internal/platform/mailer"
)

// identity is WP1+WP2 assembled: the modules that know who a caller is and what
// they may do, and the record of what they did.
type identity struct {
	audit *audit.Module
	rbac  *rbac.Module
	user  *user.Module
	auth  *auth.Module
}

// identityDeps are the infrastructure the modules are built over.
type identityDeps struct {
	Pool       *pgxpool.Pool
	Cache      rbac.PermissionCache
	Env        string
	OTPHMACKey []byte
	SMTP       mailer.SMTPConfig
}

// newIdentity constructs the modules in dependency order — audit, then rbac,
// then user, then auth. That is the order MODULE_INDEX.md §3 draws the arrows
// in: `rbac` and `user` record into `audit`, and `auth` depends on `user`.
//
// There is one circle, and it is worth naming. `audit`'s own admin operations
// are guarded by `rbac`, while `rbac` records into `audit`. Today the second
// half is not a constructor argument — `rbac` writes outbox events in the same
// transaction as its business writes (rule L4) and the worker's consumer turns
// those into audit rows — so building rbac first would compile. It would also
// quietly stop compiling the day `rbac` takes a Recorder. The guard below
// resolves its authorizer at call time instead, which breaks the circle where
// it actually is rather than where it currently is not.
func newIdentity(deps identityDeps) *identity {
	assembled := &identity{}

	assembled.audit = audit.New(audit.Deps{
		Pool: deps.Pool,

		// Resolved on the first request, by which point rbac is built. It
		// cannot be called earlier: nothing serves until this function returns.
		Guard: lazyGuard{of: assembled},

		// IPHashKey is deliberately unset. The module stores a client address
		// only as a keyed HMAC, and stores nothing at all without a key — which
		// is the right state while there is no key to give it, because an
		// unkeyed digest of an IPv4 address is reversible by anyone willing to
		// hash four billion values. There is nothing to hash yet either: no
		// request carries an authenticated actor until P2.4. Tracked in
		// internal/modules/audit/TODO.md.
		IPHashKey: nil,
	})

	assembled.rbac = rbac.New(rbac.Deps{
		Pool:  deps.Pool,
		Cache: deps.Cache,
		Env:   deps.Env,
	})

	assembled.user = user.New(user.Deps{Pool: deps.Pool})

	// `auth` comes after `user` because it holds a reference to user.Registrar.
	// The mailer renderer is built with DefaultTemplates so startup fails if a
	// template is missing from either locale — a missing template discovered at
	// request time would fail one learner's request rather than the boot.
	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		// NewRenderer fails only if a template is malformed or missing — that is
		// a programmer error, not an operational one. Panic rather than returning
		// an error that main would have to propagate through newIdentity.
		panic("mailer.NewRenderer: " + err.Error())
	}
	sender := mailer.NewSMTPSender(deps.SMTP, renderer, nil, nil)

	assembled.auth = auth.New(auth.Deps{
		Pool:       deps.Pool,
		OTPHMACKey: deps.OTPHMACKey,
		Mailer:     sender,
		Registrar:  assembled.user.Registrar(),
	})

	return assembled
}

// Routes mounts every module's operations under the caller's router.
//
// `rbac` mounts its own `/admin` group with the role guard inside it, and chi
// allows one handler per mount point — so `audit` registers its `/admin` paths
// on this router and they are wrapped here in the same guard. Both layers stay
// in force: the route group says "you are staff", and each handler still calls
// Require with the permission its own operation needs.
func (i *identity) Routes(api chi.Router) {
	i.user.Routes(api)
	i.rbac.Routes(api)
	i.auth.Routes(api)

	api.Group(func(admin chi.Router) {
		admin.Use(i.rbac.AdminOnly())
		i.audit.Routes(admin)
	})
}

// lazyGuard adapts rbac's Authorizer to the string-keyed guard `audit`
// declares, resolving it when the check runs rather than when the module is
// built.
//
// The string conversion is safe in the direction that matters: a name with no
// row in the permission catalogue is denied by the same code path that denies
// any other permission the actor lacks, so a typo fails closed.
type lazyGuard struct{ of *identity }

var _ audit.Guard = lazyGuard{}

func (g lazyGuard) Require(ctx context.Context, permission string) error {
	return g.authorizer().Require(ctx, rbaccontract.Permission(permission))
}

func (g lazyGuard) authorizer() rbaccontract.Authorizer { return g.of.rbac.Authorizer() }
