package contract

import "context"

// Exportable defines the interface for modules that hold personal data
// and must participate in GDPR-style data exports.
type Exportable interface {
	// ExportUserData returns a JSON-serializable representation of all
	// personal data this module holds for the given user.
	//
	// The returned map keys are field names; values must be JSON-serializable.
	// If the module holds no data for this user, return an empty map, not an error.
	//
	// This is called from the export job and must be idempotent.
	ExportUserData(ctx context.Context, userID string) (map[string]interface{}, error)
}
