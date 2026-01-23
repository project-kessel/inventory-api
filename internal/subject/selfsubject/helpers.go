package selfsubject

import (
	"net/url"
	"strings"
)

func (s *RedHatRbacSelfSubjectStrategy) resolveOIDCIssuerDomain(issuer string) string {
	if issuer == "" || s.oidcIssuerDomains == nil {
		return ""
	}
	if domain, ok := s.oidcIssuerDomains[issuer]; ok && domain != "" {
		return domain
	}
	if parsed, err := url.Parse(issuer); err == nil && parsed.Host != "" {
		if domain, ok := s.oidcIssuerDomains[parsed.Host]; ok && domain != "" {
			return domain
		}
	}
	return ""
}

func extractSubjectID(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	return subject
}
