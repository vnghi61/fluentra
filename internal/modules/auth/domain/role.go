package domain

// The two roles the system has, as the strings a token carries.
//
// They are declared here rather than imported from `rbac/contract` because a
// token claim is a **wire value**: once a token is signed, the string in it is
// fixed until that token expires, and it has to be readable by a verifier that
// may be running a different build. Importing rbac's type would tie the wire
// format to a Go type in another module and invite somebody to "just rename the
// constant".
//
// `auth` still asks `rbac` which roles an account holds — it does not decide
// that. These are only the names it writes down.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// HighestRole picks the role a token should carry when an account holds more
// than one.
//
// There are exactly two roles and `admin` strictly contains `user`, so the
// answer is "admin if present". The function exists rather than an inline
// check because the moment a third role is added this is the one place that has
// to stop being a boolean — and a search for callers will find it.
func HighestRole(roles []string) string {
	for _, role := range roles {
		if role == RoleAdmin {
			return RoleAdmin
		}
	}
	return RoleUser
}
