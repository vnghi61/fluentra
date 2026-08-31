package repository

import "errors"

// ErrNoPool is returned by every method on a repository built without a
// database.
//
// cmd/worker constructs modules it only needs jobs from, and a module built
// that way must fail its calls with something a caller can recognise rather
// than panicking on a nil dereference three frames down.
var ErrNoPool = errors.New("gamification repository has no database pool")
