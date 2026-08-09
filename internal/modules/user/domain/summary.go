package domain

import "github.com/google/uuid"

// Summary is the internal form of the shape other modules render. It differs
// from contract.Summary in one place on purpose: it carries the avatar's
// asset id rather than a URL, because turning an asset into a URL needs the
// storage facade and the domain imports no infrastructure.
type Summary struct {
	ID            uuid.UUID
	DisplayName   string
	AvatarAssetID *uuid.UUID
	Locale        string
	Timezone      string
	Status        Status
}
