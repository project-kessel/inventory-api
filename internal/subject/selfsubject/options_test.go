package selfsubject

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOptions_ReturnsDefaults(t *testing.T) {
	opts := NewOptions()

	assert.NotNil(t, opts)
	assert.NotNil(t, opts.RedHatRbac)
	assert.False(t, opts.RedHatRbac.Enabled)
}

func TestOptions_Validate_DisabledPasses(t *testing.T) {
	opts := NewOptions()
	opts.RedHatRbac.Enabled = false

	errs := opts.Validate()
	assert.Empty(t, errs)
}

func TestOptions_Validate_EnabledRequiresDomain(t *testing.T) {
	opts := NewOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = ""

	errs := opts.Validate()
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "issuer-domain is required")
}

func TestOptions_Validate_EnabledWithDomainPasses(t *testing.T) {
	opts := NewOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"

	errs := opts.Validate()
	assert.Empty(t, errs)
}

func TestOptions_Build_DisabledReturnsNil(t *testing.T) {
	opts := NewOptions()
	opts.RedHatRbac.Enabled = false

	strategy := opts.Build()
	assert.Nil(t, strategy)
}

func TestOptions_Build_EnabledReturnsStrategy(t *testing.T) {
	opts := NewOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"

	strategy := opts.Build()
	assert.NotNil(t, strategy)
}

func TestOptions_Build_PassesIssuerDomainMap(t *testing.T) {
	opts := NewOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"
	opts.RedHatRbac.OIDCIssuerDomainMap = map[string]string{
		"https://sso.redhat.com/auth/realms/redhat-external": "redhat",
		"https://login.example.com":                          "example",
	}

	strategy := opts.Build()
	assert.NotNil(t, strategy)

	rbacStrategy := strategy.(*RedHatRbacSelfSubjectStrategy)
	assert.Equal(t, "redhat", rbacStrategy.oidcIssuerDomains["https://sso.redhat.com/auth/realms/redhat-external"])
	assert.Equal(t, "example", rbacStrategy.oidcIssuerDomains["https://login.example.com"])
}
