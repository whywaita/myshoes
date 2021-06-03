package gh

import "strings"

// Scope is scope for auto-scaling target
type Scope int

// Scope values
const (
	Unknown Scope = iota
	Repository
	Organization
)

// String is fmt.Stringer interface
func (s Scope) String() string {
	switch s {
	case Repository:
		return "repos"
	case Organization:
		return "orgs"
	default:
		return "unknown"
	}
}

// DetectScope detect a scope (repo or org)
func DetectScope(scope string) Scope {
	sep := strings.Split(scope, "/")
	switch len(sep) {
	case 1:
		return Organization
	case 2:
		return Repository
	default:
		return Unknown
	}
}

// DivideScope divide scope to owner and repo
func DivideScope(scope string) (string, string) {
	var owner, repo string

	switch DetectScope(scope) {
	case Organization:
		owner = scope
		repo = ""
	case Repository:
		s := strings.Split(scope, "/")
		owner = s[0]
		repo = s[1]
	}

	return owner, repo
}
