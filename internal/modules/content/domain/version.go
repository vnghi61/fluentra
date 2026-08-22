package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CEFR levels defined in the OpenAPI contract.
const (
	CEFRA1 = "A1"
	CEFRA2 = "A2"
	CEFRB1 = "B1"
	CEFRB2 = "B2"
	CEFRC1 = "C1"
	CEFRC2 = "C2"
)

// ValidCEFRLevels contains all supported CEFR level codes.
var ValidCEFRLevels = map[string]bool{
	CEFRA1: true,
	CEFRA2: true,
	CEFRB1: true,
	CEFRB2: true,
	CEFRC1: true,
	CEFRC2: true,
}

// ValidateCEFRLevel checks that the CEFR level is one of A1..C2.
func ValidateCEFRLevel(level string) error {
	if !ValidCEFRLevels[level] {
		return ErrInvalidCEFRLevel.WithInternal("CEFR level must be one of A1, A2, B1, B2, C1, C2")
	}
	return nil
}

// Version represents an immutable snapshot of content material.
type Version struct {
	ID          uuid.UUID
	ItemID      uuid.UUID
	Version     int
	Kind        string
	Body        json.RawMessage
	CEFRLevel   string
	Status      AuthoringStatus
	MediaRefs   []string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
