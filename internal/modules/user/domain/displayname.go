package domain

import (
	"strings"
	"unicode"
)

// DisplayNameMinLength and DisplayNameMaxLength match the check constraint on
// core.profiles.display_name. The database is the guarantee; these exist so the
// caller gets a field-level 422 instead of a 500 from a constraint violation.
const (
	DisplayNameMinLength = 1
	DisplayNameMaxLength = 50
)

// reservedDisplayNameFragments is the BR-USER-02 impersonation list. Matching
// is on fragments, not whole names: "Fluentra Support" and "fluentra_support"
// impersonate staff exactly as well as "support" does, and a whole-name list
// would catch neither.
//
// It is deliberately blunt, and it will occasionally reject a legitimate name.
// That trade is the right way round — a learner who cannot use "Sysadmin Sam"
// can pick another name, while a learner who is convinced by "Fluentra Support"
// asking for their password cannot undo it.
var reservedDisplayNameFragments = []string{
	"fluentra",
	"admin",
	"moderator",
	"support",
	"helpdesk",
	"official",
	"staff",
	"sysop",
	"security",
}

// ValidateDisplayName enforces length and the impersonation list.
func ValidateDisplayName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return invalid("display_name", "REQUIRED", "Display name is required.")
	}

	// Counted in runes, not bytes: the database check uses char_length, and a
	// Vietnamese name is multi-byte almost everywhere.
	length := len([]rune(trimmed))
	if length < DisplayNameMinLength || length > DisplayNameMaxLength {
		return invalid("display_name", "LENGTH", "Display name must be 1 to 50 characters.")
	}

	if strings.ContainsFunc(trimmed, isDisallowedRune) {
		return invalid("display_name", "INVALID_CHARACTER", "Display name contains a character that is not allowed.")
	}

	if containsReservedFragment(trimmed) {
		return ErrDisplayNameNotAllowed.WithFields(fieldViolation(
			"display_name", "RESERVED", "That display name is not allowed."))
	}
	return nil
}

// isDisallowedRune rejects control characters and the Unicode formatting
// characters used to hide text or reverse it — a zero-width joiner or an
// override cannot be seen in a name, which is the whole reason to send one.
func isDisallowedRune(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}

// containsReservedFragment matches against a normalised form so that spacing,
// punctuation and common letter/digit substitutions do not defeat the list.
func containsReservedFragment(name string) bool {
	normalised := normaliseForReservedCheck(name)
	for _, fragment := range reservedDisplayNameFragments {
		if strings.Contains(normalised, fragment) {
			return true
		}
	}
	return false
}

// leetReplacements folds the substitutions that read as letters at a glance.
// It is not a general homoglyph defence — that belongs with the moderation
// queue, not with a synchronous validator.
var leetReplacements = strings.NewReplacer(
	"0", "o",
	"1", "l",
	"3", "e",
	"4", "a",
	"5", "s",
	"7", "t",
	"@", "a",
	"$", "s",
)

func normaliseForReservedCheck(name string) string {
	folded := leetReplacements.Replace(strings.ToLower(name))
	var builder strings.Builder
	builder.Grow(len(folded))
	for _, r := range folded {
		if unicode.IsLetter(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
