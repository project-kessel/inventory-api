package authz

import (
	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

// Authorizer type constants (kept for provider compatibility).
const (
	AllowAll = "allow-all"
	Kessel   = "kessel"
)

// CheckAuthorizerType returns the authorizer type by checking the interface.
func CheckAuthorizerType(authorizer api.Authorizer) string {
	switch authorizer.(type) {
	case *allow.AllowAllAuthz:
		return "AllowAll"
	case *kessel.KesselAuthz:
		return "Kessel"
	default:
		return "Unknown"
	}
}
