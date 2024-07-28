package util

import (
	"net/http"
	"strings"
)

func GetBearerToken(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToLower(bearer[0:6]) == "bearer" {
		return bearer[7:]
	}
	return ""
}
