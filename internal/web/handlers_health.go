package web

import (
	"log/slog"
	"net/http"

	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/doclinks"
	"github.com/exedev/docstor/internal/docs"
)

type DocHealthPageData struct {
	Summary     *docs.DocHealthSummary
	HealthPct   int
	StaleDays   int
	BrokenLinks []doclinks.BrokenLink
}

func (s *Server) handleDocHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	const staleDays = 90

	summary, err := s.docs.GetHealthSummary(ctx, tenant.ID, staleDays)
	if err != nil {
		slog.Error("failed to get health summary", "error", err)
		data := s.newPageData(r)
		data.Title = "Doc Health - Docstor"
		data.Error = "Failed to load health summary"
		s.render(w, r, "doc_health.html", data)
		return
	}

	healthPct := 0
	if summary.TotalDocs > 0 {
		healthPct = (summary.HealthyCount * 100) / summary.TotalDocs
	}

	// Load broken links
	brokenLinks, blErr := s.doclinks.GetBrokenLinks(ctx, tenant.ID)
	if blErr != nil {
		slog.Error("failed to get broken links", "error", blErr)
	}

	data := s.newPageData(r)
	data.Title = "Doc Health - Docstor"
	data.Content = DocHealthPageData{
		Summary:     summary,
		HealthPct:   healthPct,
		StaleDays:   staleDays,
		BrokenLinks: brokenLinks,
	}
	s.render(w, r, "doc_health.html", data)
}
