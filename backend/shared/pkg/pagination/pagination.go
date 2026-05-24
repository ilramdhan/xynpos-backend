package pagination

import (
	"math"

	"gorm.io/gorm"
)

const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// Request holds pagination parameters from the HTTP query string.
type Request struct {
	Page    int `query:"page" validate:"min=1"`
	PerPage int `query:"per_page" validate:"min=1,max=100"`
}

// Normalize ensures valid defaults.
func (r *Request) Normalize() {
	if r.Page < 1 {
		r.Page = DefaultPage
	}
	if r.PerPage < 1 {
		r.PerPage = DefaultPerPage
	}
	if r.PerPage > MaxPerPage {
		r.PerPage = MaxPerPage
	}
}

// Offset returns the SQL OFFSET value.
func (r *Request) Offset() int {
	return (r.Page - 1) * r.PerPage
}

// Meta holds pagination response metadata.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// NewMeta computes pagination metadata from a request and total count.
func NewMeta(req *Request, total int64) *Meta {
	req.Normalize()
	totalPages := int(math.Ceil(float64(total) / float64(req.PerPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	return &Meta{
		Page:       req.Page,
		PerPage:    req.PerPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    req.Page < totalPages,
		HasPrev:    req.Page > 1,
	}
}

// Scope returns a GORM scope that applies LIMIT + OFFSET.
//
// Usage:
//
//	db.Scopes(pagination.Scope(req)).Find(&items)
func Scope(req *Request) func(*gorm.DB) *gorm.DB {
	req.Normalize()
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(req.Offset()).Limit(req.PerPage)
	}
}

// CursorRequest holds cursor-based pagination parameters.
type CursorRequest struct {
	Cursor  string `query:"cursor"`
	PerPage int    `query:"per_page"`
}

// CursorMeta is the response metadata for cursor pagination.
type CursorMeta struct {
	PerPage    int    `json:"per_page"`
	CursorNext string `json:"cursor_next,omitempty"`
	HasNext    bool   `json:"has_next"`
}
