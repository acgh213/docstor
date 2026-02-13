// Package pagination provides shared cursor-based pagination.
package pagination

import (
	"net/http"
	"strconv"
)

const (
	DefaultPerPage = 50
	AuditPerPage   = 25
	MaxPerPage     = 200
)

// Page holds pagination state for templates.
type Page struct {
	Number  int // current page (1-based)
	PerPage int
	Total   int // total row count (-1 if unknown)
	HasNext bool
	HasPrev bool
}

// Offset returns the SQL OFFSET for this page.
func (p Page) Offset() int {
	return (p.Number - 1) * p.PerPage
}

// TotalPages returns total pages, or -1 if total is unknown.
func (p Page) TotalPages() int {
	if p.Total < 0 {
		return -1
	}
	if p.Total == 0 {
		return 1
	}
	return (p.Total + p.PerPage - 1) / p.PerPage
}

// FromRequest parses page and per_page from query string.
func FromRequest(r *http.Request, defaultPerPage int) Page {
	if defaultPerPage <= 0 {
		defaultPerPage = DefaultPerPage
	}

	page := 1
	if s := r.URL.Query().Get("page"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			page = n
		}
	}

	perPage := defaultPerPage
	if s := r.URL.Query().Get("per_page"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= MaxPerPage {
			perPage = n
		}
	}

	return Page{Number: page, PerPage: perPage, Total: -1}
}

// Apply sets Total from a count and computes HasPrev/HasNext.
func (p *Page) Apply(total int) {
	p.Total = total
	p.HasPrev = p.Number > 1
	p.HasNext = p.Number < p.TotalPages()
}

// ApplyToSlice takes a full slice and returns the page's window.
// Useful when filtering in Go (e.g., sensitivity gating).
// Sets Total, HasPrev, HasNext.
func ApplyToSlice[T any](p *Page, items []T) []T {
	p.Apply(len(items))
	start := p.Offset()
	if start >= len(items) {
		return nil
	}
	end := start + p.PerPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// PageView is the template-friendly representation including URLs.
type PageView struct {
	Number    int
	PerPage   int
	Total     int
	HasNext   bool
	HasPrev   bool
	PrevURL   string
	NextURL   string
}

// TotalPages returns total pages.
func (v PageView) TotalPages() int {
	if v.Total <= 0 {
		return 1
	}
	return (v.Total + v.PerPage - 1) / v.PerPage
}

// View builds a template-friendly PageView with prev/next URLs
// that preserve existing query parameters.
func (p Page) View(r *http.Request) PageView {
	v := PageView{
		Number:  p.Number,
		PerPage: p.PerPage,
		Total:   p.Total,
		HasNext: p.HasNext,
		HasPrev: p.HasPrev,
	}
	if p.HasPrev {
		v.PrevURL = buildPageURL(r, p.Number-1)
	}
	if p.HasNext {
		v.NextURL = buildPageURL(r, p.Number+1)
	}
	return v
}

func buildPageURL(r *http.Request, page int) string {
	q := r.URL.Query()
	if page <= 1 {
		q.Del("page")
	} else {
		q.Set("page", strconv.Itoa(page))
	}
	if r.URL.Path == "" {
		return "?" + q.Encode()
	}
	return r.URL.Path + "?" + q.Encode()
}
