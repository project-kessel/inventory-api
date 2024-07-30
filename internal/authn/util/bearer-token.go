package util

import (
	"strings"

	"github.com/go-kratos/kratos/v2/transport"
)

func GetBearerToken(t transport.Transporter) string {
	bearer := t.RequestHeader().Get("Authorization")
	if len(bearer) > 7 && strings.ToLower(bearer[0:6]) == "bearer" {
		return bearer[7:]
	}
	return ""
}
