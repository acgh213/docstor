package doclinks_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/doclinks"
	"github.com/acgh213/docstor/internal/testutil"
)

// --- Unit tests for link parsing ---

func TestParseLinks_AbsoluteDocLinks(t *testing.T) {
	md := `Check [firewall rules](/docs/network/firewall) and [VPN config](/docs/network/vpn).`
	links := doclinks.ParseLinks(md)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %v", len(links), links)
	}
	if links[0] != "network/firewall" {
		t.Errorf("expected network/firewall, got %s", links[0])
	}
	if links[1] != "network/vpn" {
		t.Errorf("expected network/vpn, got %s", links[1])
	}
}

func TestParseLinks_MarkdownSuffix(t *testing.T) {
	md := `See [setup](setup.md) for details.`
	links := doclinks.ParseLinks(md)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0] != "setup" {
		t.Errorf("expected setup, got %s", links[0])
	}
}

func TestParseLinks_WithAnchors(t *testing.T) {
	md := `See [section](/docs/guide/intro#setup) for details.`
	links := doclinks.ParseLinks(md)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0] != "guide/intro" {
		t.Errorf("expected guide/intro, got %s", links[0])
	}
}

func TestParseLinks_Deduplication(t *testing.T) {
	md := `[link1](/docs/foo) and [link2](/docs/foo) are the same.`
	links := doclinks.ParseLinks(md)
	if len(links) != 1 {
		t.Fatalf("expected 1 deduplicated link, got %d: %v", len(links), links)
	}
}

func TestParseLinks_IgnoresExternalLinks(t *testing.T) {
	md := `See [Google](https://google.com) and [GitHub](http://github.com).`
	links := doclinks.ParseLinks(md)
	if len(links) != 0 {
		t.Fatalf("expected 0 links, got %d: %v", len(links), links)
	}
}

func TestNormalizeLinkPath(t *testing.T) {
	cases := []struct {
		input, expected string
	}{
		{"/docs/network/firewall", "network/firewall"},
		{"setup.md", "setup"},
		{"/docs/guide#section", "guide"},
		{"/docs/a/../b", "b"},
		{"", ""},
	}
	for _, tc := range cases {
		got := doclinks.NormalizeLinkPath(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeLinkPath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestResolveRelative(t *testing.T) {
	cases := []struct {
		from, link, expected string
	}{
		{"network/firewall", "vpn", "network/vpn"},
		{"network/firewall", "/docs/other", "docs/other"},
		{"top-level", "sub", "sub"},
	}
	for _, tc := range cases {
		got := doclinks.ResolveRelative(tc.from, tc.link)
		if got != tc.expected {
			t.Errorf("ResolveRelative(%q, %q) = %q, want %q", tc.from, tc.link, got, tc.expected)
		}
	}
}

// --- Integration tests ---

func TestRebuildLinks_CreatesLinks(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "DocLinks-Links")
	uid := testutil.CreateUser(t, pool, "doclinks-links@test.com", "doclinks-links", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	// Create two documents
	var docA, docB uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'network/firewall', 'Firewall', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docA)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'network/vpn', 'VPN', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docB)
	if err != nil {
		t.Fatal(err)
	}

	repo := doclinks.NewRepository(pool)

	// Doc A links to Doc B
	md := `See [VPN config](/docs/network/vpn) for details.`
	err = repo.RebuildLinks(context.Background(), tid, docA, "network/firewall", md)
	if err != nil {
		t.Fatal(err)
	}

	// Verify backlinks: Doc B should have Doc A as a backlink
	backlinks, err := repo.GetBacklinks(context.Background(), tid, docB)
	if err != nil {
		t.Fatal(err)
	}
	if len(backlinks) != 1 {
		t.Fatalf("expected 1 backlink, got %d", len(backlinks))
	}
	if backlinks[0].DocumentID != docA {
		t.Errorf("expected backlink from %s, got %s", docA, backlinks[0].DocumentID)
	}
	if backlinks[0].Title != "Firewall" {
		t.Errorf("expected title Firewall, got %s", backlinks[0].Title)
	}

	// No broken links
	broken, err := repo.GetBrokenLinks(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(broken) != 0 {
		t.Errorf("expected 0 broken links, got %d", len(broken))
	}
}

func TestRebuildLinks_BrokenLinks(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "DocLinks-Broken")
	uid := testutil.CreateUser(t, pool, "doclinks-broken@test.com", "doclinks-broken", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	var docA uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'guides/setup', 'Setup', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docA)
	if err != nil {
		t.Fatal(err)
	}

	repo := doclinks.NewRepository(pool)

	// Doc A links to a non-existent path
	md := `See [missing doc](/docs/guides/nonexistent) for details.`
	err = repo.RebuildLinks(context.Background(), tid, docA, "guides/setup", md)
	if err != nil {
		t.Fatal(err)
	}

	// Should have 1 broken link
	broken, err := repo.GetBrokenLinks(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(broken) != 1 {
		t.Fatalf("expected 1 broken link, got %d", len(broken))
	}
	if broken[0].LinkPath != "guides/nonexistent" {
		t.Errorf("expected link path guides/nonexistent, got %s", broken[0].LinkPath)
	}

	count, err := repo.CountBrokenLinks(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestRebuildLinks_Idempotent(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "DocLinks-Idempotent")
	uid := testutil.CreateUser(t, pool, "doclinks-idempotent@test.com", "doclinks-idempotent", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	var docA, docB uuid.UUID
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'a', 'Doc A', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docA)
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'b', 'Doc B', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docB)

	repo := doclinks.NewRepository(pool)

	md := `Link to [B](/docs/b).`
	_ = repo.RebuildLinks(context.Background(), tid, docA, "a", md)
	_ = repo.RebuildLinks(context.Background(), tid, docA, "a", md)
	_ = repo.RebuildLinks(context.Background(), tid, docA, "a", md)

	// Should still have exactly 1 backlink
	backlinks, _ := repo.GetBacklinks(context.Background(), tid, docB)
	if len(backlinks) != 1 {
		t.Fatalf("expected 1 backlink after 3 rebuilds, got %d", len(backlinks))
	}
}

func TestDocLinks_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid1 := testutil.CreateTenant(t, pool, "DocLinks-T1")
	tid2 := testutil.CreateTenant(t, pool, "DocLinks-T2")
	uid1 := testutil.CreateUser(t, pool, "doclinks-t1@test.com", "doclinks-t1", "password123")
	uid2 := testutil.CreateUser(t, pool, "doclinks-t2@test.com", "doclinks-t2", "password123")
	testutil.CreateMembership(t, pool, tid1, uid1, "editor")
	testutil.CreateMembership(t, pool, tid2, uid2, "editor")

	var docA1, docB1 uuid.UUID
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'shared/path', 'T1 Doc A', 'doc', 'public-internal', $2) RETURNING id`,
		tid1, uid1).Scan(&docA1)
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'shared/target', 'T1 Doc B', 'doc', 'public-internal', $2) RETURNING id`,
		tid1, uid1).Scan(&docB1)

	var docA2 uuid.UUID
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'shared/path', 'T2 Doc A', 'doc', 'public-internal', $2) RETURNING id`,
		tid2, uid2).Scan(&docA2)

	repo := doclinks.NewRepository(pool)

	// T1: Doc A links to Doc B
	_ = repo.RebuildLinks(context.Background(), tid1, docA1, "shared/path", `[link](/docs/shared/target)`)

	// T2: Doc A links to the same path (but it doesn't exist in T2)
	_ = repo.RebuildLinks(context.Background(), tid2, docA2, "shared/path", `[link](/docs/shared/target)`)

	// T1 backlinks should show T1's link
	bl1, _ := repo.GetBacklinks(context.Background(), tid1, docB1)
	if len(bl1) != 1 {
		t.Errorf("expected 1 backlink in T1, got %d", len(bl1))
	}

	// T2 should have a broken link (target doesn't exist in T2)
	broken2, _ := repo.GetBrokenLinks(context.Background(), tid2)
	if len(broken2) != 1 {
		t.Errorf("expected 1 broken link in T2, got %d", len(broken2))
	}

	// T1 should have 0 broken links
	broken1, _ := repo.GetBrokenLinks(context.Background(), tid1)
	if len(broken1) != 0 {
		t.Errorf("expected 0 broken links in T1, got %d", len(broken1))
	}
}

func TestMarkLinksToPathBroken(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "DocLinks-MarkBroken")
	uid := testutil.CreateUser(t, pool, "doclinks-markbroken@test.com", "doclinks-markbroken", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	var docA, docB uuid.UUID
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'from-doc', 'From', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docA)
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'target-doc', 'Target', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docB)

	repo := doclinks.NewRepository(pool)

	// Build link from A -> B
	_ = repo.RebuildLinks(context.Background(), tid, docA, "from-doc", `[Target](/docs/target-doc)`)

	// Verify link is not broken
	broken, _ := repo.GetBrokenLinks(context.Background(), tid)
	if len(broken) != 0 {
		t.Fatalf("expected 0 broken links, got %d", len(broken))
	}

	// Mark links to target-doc as broken (simulating rename/delete)
	err := repo.MarkLinksToPathBroken(context.Background(), tid, "target-doc")
	if err != nil {
		t.Fatal(err)
	}

	// Now should have 1 broken link
	broken, _ = repo.GetBrokenLinks(context.Background(), tid)
	if len(broken) != 1 {
		t.Fatalf("expected 1 broken link after mark, got %d", len(broken))
	}
}
