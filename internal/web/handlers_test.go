package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/config"
	"github.com/acgh213/docstor/internal/docs"
	"github.com/acgh213/docstor/internal/testutil"
	"github.com/acgh213/docstor/internal/web"
)

// newTestServer builds a real router backed by the test database.
func newTestServer(t *testing.T) (http.Handler, *testutil.TestEnv) {
	t.Helper()
	pool := testutil.SetupDB(t)

	cfg := &config.Config{
		Env:                   "development",
		Port:                  "0",
		DatabaseURL:           testutil.TestDatabaseURL(),
		SessionKey:            "test-session-key-1234567890abcdef",
		CSRFKey:               "test-csrf-key-1234567890abcdefgh",
		AttachmentStoragePath: t.TempDir(),
	}

	router := web.NewRouter(pool, cfg)

	env := &testutil.TestEnv{Pool: pool}
	return router, env
}

// doRequest issues a request with optional session cookie and returns the response.
// For POST requests it automatically obtains a CSRF token first via a GET to the same path.
func doRequest(t *testing.T, handler http.Handler, method, path string, form url.Values, sessionToken string) *httptest.ResponseRecorder {
	t.Helper()

	var csrfToken string
	var csrfCookies []*http.Cookie

	// For POST/PUT/PATCH/DELETE, we need a valid CSRF token.
	if method != "GET" && method != "HEAD" {
		// Issue a GET to a known page to obtain CSRF cookie + token.
		// We use "/" (dashboard) because the target path might 403/404 for this user.
		getReq := httptest.NewRequest("GET", "/", nil)
		if sessionToken != "" {
			getReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionToken})
		}
		getRR := httptest.NewRecorder()
		handler.ServeHTTP(getRR, getReq)

		// Extract csrf_token cookie.
		csrfCookies = getRR.Result().Cookies()

		// Extract the masked CSRF token from the HTML body.
		// nosurf embeds it via template as <input type="hidden" name="csrf_token" value="...">
		re := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
		if m := re.FindStringSubmatch(getRR.Body.String()); len(m) == 2 {
			csrfToken = m[1]
		}

		if form == nil {
			form = url.Values{}
		}
		form.Set("csrf_token", csrfToken)
	}

	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	// nosurf v1.2.0 validates Origin/Referer on mutation requests.
	if method != "GET" && method != "HEAD" {
		req.Header.Set("Origin", "http://example.com")
	}
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionToken})
	}
	for _, c := range csrfCookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// Role gating tests
// ---------------------------------------------------------------------------

func TestRoleGating_ReaderCannotCreateDoc(t *testing.T) {
	router, env := newTestServer(t)

	reader := testutil.QuickFixture(t, env.Pool, "RoleTest", "reader@role.com", "reader")

	// GET /docs/new — reader should get 403
	rr := doRequest(t, router, "GET", "/docs/new", nil, reader.SessionToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("GET /docs/new: got %d, want 403", rr.Code)
	}

	// POST /docs/new — reader should get 403
	form := url.Values{
		"path":  {"reader-doc"},
		"title": {"Reader Doc"},
		"body":  {"body"},
	}
	rr = doRequest(t, router, "POST", "/docs/new", form, reader.SessionToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST /docs/new: got %d, want 403", rr.Code)
	}
}

func TestRoleGating_ReaderCannotEditDoc(t *testing.T) {
	router, env := newTestServer(t)

	// Create an editor to make the doc, and a reader to test access.
	editor := testutil.QuickFixture(t, env.Pool, "RoleEdit", "editor@role.com", "editor")
	// Reader in same tenant.
	readerUID := testutil.CreateUser(t, env.Pool, "reader2@role.com", "Reader2", "password123")
	testutil.CreateMembership(t, env.Pool, editor.TenantID, readerUID, "reader")
	readerToken := testutil.CreateSession(t, env.Pool, readerUID, editor.TenantID)

	// Editor creates a doc.
	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      "edit-gate",
		Title:     "Edit Gate",
		CreatedBy: editor.UserID,
		Body:      "original",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	// Reader tries to GET edit page.
	rr := doRequest(t, router, "GET", "/docs/id/"+doc.ID.String()+"/edit", nil, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("GET edit: got %d, want 403", rr.Code)
	}

	// Reader tries to POST save.
	form := url.Values{
		"body":             {"hacked"},
		"message":          {"pwned"},
		"base_revision_id": {doc.CurrentRevisionID.String()},
	}
	rr = doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/save", form, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST save: got %d, want 403", rr.Code)
	}

	// Reader tries to POST revert.
	rr = doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/revert/"+doc.CurrentRevisionID.String(), nil, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST revert: got %d, want 403", rr.Code)
	}
}

func TestRoleGating_EditorCanCreateAndEdit(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "EditorTest", "ed@edit.com", "editor")

	// GET /docs/new should work (200).
	rr := doRequest(t, router, "GET", "/docs/new", nil, editor.SessionToken)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /docs/new: got %d, want 200", rr.Code)
	}

	// POST /docs/new should succeed (redirect 303).
	form := url.Values{
		"path":    {"editor-created"},
		"title":   {"Editor Doc"},
		"body":    {"editor body"},
		"message": {"init"},
	}
	rr = doRequest(t, router, "POST", "/docs/new", form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /docs/new: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestRoleGating_AdminCanDoEverything(t *testing.T) {
	router, env := newTestServer(t)
	admin := testutil.QuickFixture(t, env.Pool, "AdminTest", "admin@admin.com", "admin")

	// Admin can GET /docs/new.
	rr := doRequest(t, router, "GET", "/docs/new", nil, admin.SessionToken)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /docs/new: got %d, want 200", rr.Code)
	}

	// Admin can POST /docs/new.
	form := url.Values{
		"path":    {"admin-doc"},
		"title":   {"Admin Doc"},
		"body":    {"admin body"},
		"message": {"init"},
	}
	rr = doRequest(t, router, "POST", "/docs/new", form, admin.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /docs/new: got %d, want 303", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Sensitivity gating via HTTP
// ---------------------------------------------------------------------------

func TestSensitivityGating_ReaderCannotAccessRestricted(t *testing.T) {
	router, env := newTestServer(t)

	// Admin creates a restricted doc.
	admin := testutil.QuickFixture(t, env.Pool, "SensTest", "admin@sens.com", "admin")
	readerUID := testutil.CreateUser(t, env.Pool, "reader@sens.com", "SensReader", "password123")
	testutil.CreateMembership(t, env.Pool, admin.TenantID, readerUID, "reader")
	readerToken := testutil.CreateSession(t, env.Pool, readerUID, admin.TenantID)

	// Insert restricted doc via repo.
	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:    admin.TenantID,
		Path:        "restricted-doc",
		Title:       "Restricted",
		Sensitivity: docs.SensitivityRestricted,
		CreatedBy:   admin.UserID,
		Body:        "secret stuff",
		Message:     "init",
	})
	if err != nil {
		t.Fatalf("create restricted doc: %v", err)
	}

	// Reader tries to read it by path.
	rr := doRequest(t, router, "GET", "/docs/restricted-doc", nil, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("GET restricted doc by path: got %d, want 403", rr.Code)
	}

	// Reader tries to view history.
	rr = doRequest(t, router, "GET", "/docs/id/"+doc.ID.String()+"/history", nil, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("GET restricted doc history: got %d, want 403", rr.Code)
	}

	// Admin should see it fine.
	rr = doRequest(t, router, "GET", "/docs/restricted-doc", nil, admin.SessionToken)
	if rr.Code != http.StatusOK {
		t.Errorf("admin GET restricted doc: got %d, want 200", rr.Code)
	}
}

// suppress unused import warnings
var (
	_ = uuid.Nil
)
