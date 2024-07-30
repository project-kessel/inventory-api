package middleware

import (
	"context"
	"net/http"
	"strconv"

	"gorm.io/gorm"
)

type PagedReponseMetadata struct {
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Total int64 `json:"total"`
}

type PagedResponse[R any] struct {
	PagedReponseMetadata
	Items []R `json:"items"`
}

type PaginationRequest struct {
	Page    int
	MaxSize int
	Filter  func(*gorm.DB) *gorm.DB
}

func Pagination(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		var page int = 1
		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		sizeStr := r.URL.Query().Get("size")
		var size int = 10
		if sizeStr != "" {
			if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
				size = s
			}
		}

		if size > 100 {
			size = 100
		}

		filter := func(db *gorm.DB) *gorm.DB {
			return db.Offset((page - 1) * size).Limit(size)
		}

		paginationRequest := &PaginationRequest{
			Page:    page,
			MaxSize: size,
			Filter:  filter,
		}

		ctx := context.WithValue(r.Context(), PaginationRequestKey, paginationRequest)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var (
	PaginationRequestKey = &contextKey{"paginationRequest"}
	GetPaginationRequest = GetFromContext[PaginationRequest](PaginationRequestKey)
)
