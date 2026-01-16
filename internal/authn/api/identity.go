package api

// Identity is the identity of the requester
type Identity struct {
	UserID string `yaml:"user_id"`
	// TODO: do we even need the tenant id?
	Tenant string `yaml:"tenant"`

	Principal string   `yaml:"principal"`
	Groups    []string `yaml:"groups"`

	// TODO: If we explicitly represent reporters in the database, do we need to distinguish them when they authenticate?
	IsReporter bool   `yaml:"is_reporter"`
	Type       string `yaml:"type"`
	Href       string `yaml:"href"`
	IsGuest    bool   `yaml:"is_guest"`

	// AuthType indicates the authentication method used (e.g., "oidc", "x-rh-identity", "allow-unauthenticated")
	// This is used by authorization middleware to make access control decisions
	AuthType string `yaml:"auth_type"`
}
