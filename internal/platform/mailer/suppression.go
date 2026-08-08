package mailer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SuppressionStore records addresses that must not be emailed again.
type SuppressionStore interface {
	IsSuppressed(ctx context.Context, email string) (bool, string, error)
	SuppressAddress(ctx context.Context, email, reason string) error
}

// DB is the subset of a pgx pool this package needs.
type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// HashEmail computes the SHA-256 hash of a normalized email address. Only the
// hash is stored: a suppression list of plaintext addresses is a mailing list.
func HashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// PostgresSuppressionStore persists suppressions in comm.email_suppressions.
// A hard bounce has to outlive the process that observed it; an in-memory list
// silently starts emailing a dead address again after every deploy.
type PostgresSuppressionStore struct {
	db DB
}

// NewPostgresSuppressionStore creates a durable suppression store.
func NewPostgresSuppressionStore(db DB) *PostgresSuppressionStore {
	return &PostgresSuppressionStore{db: db}
}

// IsSuppressed reports whether the address is on the suppression list, and why.
func (s *PostgresSuppressionStore) IsSuppressed(
	ctx context.Context, email string,
) (bool, string, error) {
	const query = `SELECT reason FROM comm.email_suppressions WHERE email_hash = $1`
	var reason string
	err := s.db.QueryRow(ctx, query, HashEmail(email)).Scan(&reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("look up email suppression: %w", err)
	}
	return true, reason, nil
}

// SuppressAddress adds the address to the suppression list, or updates the
// reason if it is already there.
func (s *PostgresSuppressionStore) SuppressAddress(ctx context.Context, email, reason string) error {
	if strings.TrimSpace(email) == "" {
		return errors.New("mailer: email address cannot be empty")
	}
	const query = `
		INSERT INTO comm.email_suppressions (email_hash, reason)
		VALUES ($1, $2)
		ON CONFLICT (email_hash) DO UPDATE SET reason = EXCLUDED.reason
	`
	if _, err := s.db.Exec(ctx, query, HashEmail(email), reason); err != nil {
		return fmt.Errorf("suppress email address: %w", err)
	}
	return nil
}

var _ SuppressionStore = (*PostgresSuppressionStore)(nil)

// MemorySuppressionStore is for tests and for the dev stack, where no database
// is required. It is deliberately not the default in production wiring.
type MemorySuppressionStore struct {
	mu           sync.RWMutex
	suppressions map[string]string
}

// NewMemorySuppressionStore initializes an in-memory suppression store.
func NewMemorySuppressionStore() *MemorySuppressionStore {
	return &MemorySuppressionStore{suppressions: make(map[string]string)}
}

// IsSuppressed reports whether the address is on the in-memory list, and why.
func (s *MemorySuppressionStore) IsSuppressed(_ context.Context, email string) (bool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reason, found := s.suppressions[HashEmail(email)]
	return found, reason, nil
}

// SuppressAddress adds the address to the in-memory list. It is lost on restart,
// which is exactly why PostgresSuppressionStore is the production wiring.
func (s *MemorySuppressionStore) SuppressAddress(_ context.Context, email, reason string) error {
	if strings.TrimSpace(email) == "" {
		return errors.New("mailer: email address cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suppressions[HashEmail(email)] = reason
	return nil
}

var _ SuppressionStore = (*MemorySuppressionStore)(nil)
