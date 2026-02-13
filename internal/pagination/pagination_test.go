package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFromRequest_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/items", nil)
	pg := FromRequest(r, DefaultPerPage)
	if pg.Number != 1 {
		t.Errorf("expected page 1, got %d", pg.Number)
	}
	if pg.PerPage != DefaultPerPage {
		t.Errorf("expected per_page %d, got %d", DefaultPerPage, pg.PerPage)
	}
}

func TestFromRequest_CustomPage(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?page=3&per_page=25", nil)
	pg := FromRequest(r, DefaultPerPage)
	if pg.Number != 3 {
		t.Errorf("expected page 3, got %d", pg.Number)
	}
	if pg.PerPage != 25 {
		t.Errorf("expected per_page 25, got %d", pg.PerPage)
	}
}

func TestFromRequest_InvalidPage(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?page=-1&per_page=abc", nil)
	pg := FromRequest(r, DefaultPerPage)
	if pg.Number != 1 {
		t.Errorf("expected page 1, got %d", pg.Number)
	}
	if pg.PerPage != DefaultPerPage {
		t.Errorf("expected default per_page, got %d", pg.PerPage)
	}
}

func TestFromRequest_MaxPerPage(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?per_page=9999", nil)
	pg := FromRequest(r, DefaultPerPage)
	if pg.PerPage != DefaultPerPage {
		t.Errorf("expected default per_page (clamped), got %d", pg.PerPage)
	}
}

func TestApply(t *testing.T) {
	pg := Page{Number: 2, PerPage: 10, Total: -1}
	pg.Apply(25)
	if pg.Total != 25 {
		t.Errorf("expected total 25, got %d", pg.Total)
	}
	if !pg.HasPrev {
		t.Error("expected HasPrev true")
	}
	if !pg.HasNext {
		t.Error("expected HasNext true")
	}
	if pg.TotalPages() != 3 {
		t.Errorf("expected 3 total pages, got %d", pg.TotalPages())
	}
}

func TestApplyToSlice(t *testing.T) {
	items := make([]int, 123)
	for i := range items {
		items[i] = i
	}

	pg := Page{Number: 1, PerPage: 50, Total: -1}
	result := ApplyToSlice(&pg, items)
	if len(result) != 50 {
		t.Errorf("expected 50 items, got %d", len(result))
	}
	if result[0] != 0 {
		t.Errorf("expected first item 0, got %d", result[0])
	}
	if pg.Total != 123 {
		t.Errorf("expected total 123, got %d", pg.Total)
	}
	if pg.HasPrev {
		t.Error("page 1 should not have prev")
	}
	if !pg.HasNext {
		t.Error("expected HasNext true")
	}

	// Page 3 (last)
	pg2 := Page{Number: 3, PerPage: 50, Total: -1}
	result2 := ApplyToSlice(&pg2, items)
	if len(result2) != 23 {
		t.Errorf("expected 23 items on last page, got %d", len(result2))
	}
	if result2[0] != 100 {
		t.Errorf("expected first item 100, got %d", result2[0])
	}
	if !pg2.HasPrev {
		t.Error("expected HasPrev true")
	}
	if pg2.HasNext {
		t.Error("last page should not have next")
	}
}

func TestApplyToSlice_BeyondRange(t *testing.T) {
	items := []int{1, 2, 3}
	pg := Page{Number: 99, PerPage: 50, Total: -1}
	result := ApplyToSlice(&pg, items)
	if result != nil {
		t.Errorf("expected nil for out-of-range page, got %v", result)
	}
}

func TestView_URLs(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/docs?client=abc&type=doc", nil)
	pg := Page{Number: 2, PerPage: 50}
	pg.Apply(150)
	v := pg.View(r)

	if v.PrevURL == "" {
		t.Error("expected prev URL")
	}
	if v.NextURL == "" {
		t.Error("expected next URL")
	}
	// Prev URL should go to page 1, which should not have a page param
	if v.PrevURL != "/docs?client=abc&type=doc" {
		t.Errorf("unexpected prev URL: %s", v.PrevURL)
	}
	// Next URL should have page=3
	if v.NextURL != "/docs?client=abc&page=3&type=doc" {
		t.Errorf("unexpected next URL: %s", v.NextURL)
	}
}

func TestView_FirstPage_NoPrevURL(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/docs", nil)
	pg := Page{Number: 1, PerPage: 50}
	pg.Apply(25)
	v := pg.View(r)
	if v.HasPrev {
		t.Error("first page should not have prev")
	}
	if v.HasNext {
		t.Error("single page should not have next")
	}
	if v.PrevURL != "" {
		t.Error("expected empty prev URL")
	}
}
