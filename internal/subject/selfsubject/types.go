package selfsubject

// RedHatRbacSelfSubjectStrategyConfig configures the Red Hat RBAC strategy.
type RedHatRbacSelfSubjectStrategyConfig struct {
	Enabled             bool
	XRhIdentityDomain   string
	OIDCIssuerDomainMap map[string]string
}
