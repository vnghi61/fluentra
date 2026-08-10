package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/secret"
)

// Credential is one account's password, as stored.
//
// PasswordHash is wrapped rather than a plain string because of what this
// module's AGENT.md §12 asks for and because of where the struct travels: a
// `%+v` in a log line, a span attribute, or a test failure message would
// otherwise print the digest, its salt and its parameters. Reading it takes an
// explicit Reveal, which is exactly one call site — the verifier.
type Credential struct {
	ID     uuid.UUID
	UserID uuid.UUID

	PasswordHash secret.Redacted[string]

	// AlgoParams is the `m=…,t=…,p=…` segment, computed by the database from
	// PasswordHash. It is here so a caller can see which parameters a stored
	// hash used without decoding it, and it is read-only: writing to it changes
	// nothing, because the column is generated.
	AlgoParams string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Verify checks password against this credential and reports whether the stored
// hash should be replaced because it was made with superseded parameters.
//
// The rehash decision lives here rather than in the caller so that the two
// things that must stay in step — the parameters new hashes use and the
// parameters an old hash is measured against — are the same value, read once.
func (c Credential) Verify(hasher Hasher, password string) (Verification, error) {
	return hasher.Verify(password, c.PasswordHash.Reveal())
}
