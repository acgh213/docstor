package cmdb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/exedev/docstor/internal/cmdb"
	"github.com/exedev/docstor/internal/testutil"
)

func TestShortcodes_ValidSystem(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "SCTest", "sc@sc.com", "editor")

	sys, err := repo.CreateSystem(ctx, cmdb.CreateSystemInput{
		TenantID:   f.TenantID,
		SystemType: "server",
		Name:       "web-prod",
		IP:         "10.0.0.5",
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	input := "<p>Check {{system:" + sys.ID.String() + "}} for status</p>"
	result := repo.RenderShortcodes(ctx, f.TenantID, input)

	if strings.Contains(result, "{{system:") {
		t.Error("shortcode was not replaced")
	}
	if !strings.Contains(result, "web-prod") {
		t.Error("rendered output should contain system name")
	}
	if !strings.Contains(result, "10.0.0.5") {
		t.Error("rendered output should contain IP")
	}
}

func TestShortcodes_MissingEntity(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "SCMissing", "scm@sc.com", "editor")

	fakeID := uuid.New().String()
	input := "<p>Check {{system:" + fakeID + "}}</p>"
	result := repo.RenderShortcodes(ctx, f.TenantID, input)

	if strings.Contains(result, "{{system:") {
		t.Error("shortcode was not replaced")
	}
	if !strings.Contains(result, "not found") {
		t.Error("missing entity should show 'not found' warning")
	}
}

func TestShortcodes_XSSEscaped(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := cmdb.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "SCXss", "scx@sc.com", "editor")

	// Create system with XSS payload in name
	sys, err := repo.CreateSystem(ctx, cmdb.CreateSystemInput{
		TenantID:   f.TenantID,
		SystemType: "server",
		Name:       `<script>alert('xss')</script>`,
		IP:         "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("CreateSystem: %v", err)
	}

	input := "{{system:" + sys.ID.String() + "}}"
	result := repo.RenderShortcodes(ctx, f.TenantID, input)

	if strings.Contains(result, "<script>") {
		t.Error("XSS payload not escaped in shortcode output")
	}
}
