package id

import (
	"context"
	"crypto/rand"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// NewUUIDv7 creates a time-ordered UUIDv7.
func NewUUIDv7(ctx context.Context) (uuid.UUID, error) {
	if err := ctx.Err(); err != nil {
		return uuid.Nil, err
	}
	return uuid.NewV7()
}

// NewULID creates a sortable ULID using cryptographic randomness.
func NewULID(ctx context.Context) (ulid.ULID, error) {
	if err := ctx.Err(); err != nil {
		return ulid.ULID{}, err
	}
	return ulid.New(ulid.Now(), rand.Reader)
}
