package domain

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Known taxonomy namespaces.
const (
	NamespaceCourseTopic = "course_topic"
	// NamespaceContentTag is the namespace content's own tag resolver writes.
	// It predates the spine and is not one of its strands.
	NamespaceContentTag    = "topic"
	NamespaceGrammar       = "grammar"
	NamespaceVocabulary    = "vocabulary"
	NamespacePattern       = "pattern"
	NamespacePronunciation = "pronunciation"
	NamespaceSkill         = "skill"
)

var validNamespaces = map[string]bool{
	NamespaceCourseTopic:   true,
	NamespaceContentTag:    true,
	NamespaceGrammar:       true,
	NamespaceVocabulary:    true,
	NamespacePattern:       true,
	NamespacePronunciation: true,
	NamespaceSkill:         true,
}

var (
	reScreamingSnake = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)
	reKebabCase      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// Taxonomy represents a controlled classification entry (topic, skill, exam, etc.).
type Taxonomy struct {
	ID           uuid.UUID
	Namespace    string
	Code         string
	Label        string
	ParentID     *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Description  string
	CEFRLevel    *string
	Position     int
	DeprecatedAt *time.Time
}

// IsDeprecated reports whether the taxonomy entry has been deprecated.
func (t Taxonomy) IsDeprecated() bool {
	return t.DeprecatedAt != nil
}

// ValidateNamespace checks whether namespace is one of the recognized namespaces.
func ValidateNamespace(ns string) bool {
	return validNamespaces[ns]
}

// ValidateTaxonomyCode validates code format per namespace.
func ValidateTaxonomyCode(namespace, code string) bool {
	if namespace == NamespaceCourseTopic || namespace == NamespaceContentTag {
		return reKebabCase.MatchString(code)
	}
	return reScreamingSnake.MatchString(code)
}

// TaxonomyTag is the lightweight representation used in content responses.
type TaxonomyTag struct {
	Namespace string
	Code      string
	Label     string
}
