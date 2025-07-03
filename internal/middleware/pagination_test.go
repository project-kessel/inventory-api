package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestPaginationMiddleware(t *testing.T) {
	type testCase struct {
		name      string
		pageParam string
		sizeParam string
		wantPage  int
		wantSize  int
	}

	tests := []testCase{
		{"defaults", "", "", 1, 10},
		{"custom values", "3", "20", 3, 20},
		{"invalid values", "bad", "bad", 1, 10},
		{"negative values", "-5", "-8", 1, 10},
		{"zero values", "0", "0", 1, 10},
		{"too large size", "1", "999", 1, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				val := r.Context().Value(middleware.PaginationRequestKey)
				assert.NotNil(t, val)
				pagination, ok := val.(*middleware.PaginationRequest)
				assert.True(t, ok)
				assert.Equal(t, tc.wantPage, pagination.Page)
				assert.Equal(t, tc.wantSize, pagination.MaxSize)
			})

			paginated := middleware.Pagination(handler)

			req := httptest.NewRequest("GET", "/?page="+tc.pageParam+"&size="+tc.sizeParam, nil)
			rr := httptest.NewRecorder()
			paginated.ServeHTTP(rr, req)
		})
	}
}
