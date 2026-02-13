package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/testutil"
)

// createUserWithKnownEmail creates a user with a predictable email so we can login.
func createUserWithKnownEmail(t *testing.T, env *testutil.TestEnv, prefix string) (uuid.UUID, string) {
	t.Helper()
	email := fmt.Sprintf("%s.%s@session.test", uuid.New().String()[:8], prefix)
	hash, _ := auth.HashPassword("password123")
	var uid uuid.UUID
	err := env.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, prefix, hash).Scan(&uid)
	if err != nil {
		t.Fatal(err)
	}
	return uid, email
}

// loginAndGetSession performs a full login flow and returns the session cookie.
func loginAndGetSession(t *testing.T, router http.Handler, email, password string) *http.Cookie {
	t.Helper()

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	csrfRe := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
	matches := csrfRe.FindStringSubmatch(rr.Body.String())
	if len(matches) < 2 {
		t.Fatal("no CSRF token on login page")
	}
	cookies := rr.Result().Cookies()

	form := url.Values{"email": {email}, "password": {password}, "csrf_token": {matches[1]}}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("login failed: %d", rr.Code)
	}

	for _, c := range rr.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie after login")
	return nil
}

func TestSession_LoginCreatesSession(t *testing.T) {
	router, env := newTestServer(t)
	tid := testutil.CreateTenant(t, env.Pool, "Session-Login")
	uid, email := createUserWithKnownEmail(t, env, "login")
	testutil.CreateMembership(t, env.Pool, tid, uid, "editor")

	sessionCookie := loginAndGetSession(t, router, email, "password123")

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for authenticated page, got %d", rr.Code)
	}
}

func TestSession_LogoutDestroysSession(t *testing.T) {
	router, env := newTestServer(t)
	tid := testutil.CreateTenant(t, env.Pool, "Session-Logout")
	uid, email := createUserWithKnownEmail(t, env, "logout")
	testutil.CreateMembership(t, env.Pool, tid, uid, "editor")

	sessionCookie := loginAndGetSession(t, router, email, "password123")

	// Get CSRF from an authenticated page
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	csrfRe := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
	matches := csrfRe.FindStringSubmatch(rr.Body.String())
	if len(matches) < 2 {
		t.Fatal("no CSRF token on authenticated page")
	}
	allCookies := rr.Result().Cookies()

	form := url.Values{"csrf_token": {matches[1]}}
	req = httptest.NewRequest("POST", "/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	req.AddCookie(sessionCookie)
	for _, c := range allCookies {
		req.AddCookie(c)
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on logout, got %d", rr.Code)
	}

	// Old session cookie should no longer work on protected routes
	req = httptest.NewRequest("GET", "/docs", nil)
	req.AddCookie(sessionCookie)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Errorf("expected redirect to login after logout, got %d", rr.Code)
	}
}

func TestSession_ExpiredSessionRejected(t *testing.T) {
	router, env := newTestServer(t)
	tid := testutil.CreateTenant(t, env.Pool, "Session-Expired")
	uid, _ := createUserWithKnownEmail(t, env, "expired")
	testutil.CreateMembership(t, env.Pool, tid, uid, "editor")

	ctx := context.Background()
	sm := auth.NewSessionManager(env.Pool)
	token, err := sm.Create(ctx, uid, tid, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatal(err)
	}

	// Expire it by setting expires_at in the past for this specific user
	_, err = env.Pool.Exec(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE user_id = $2`,
		time.Now().Add(-time.Hour), uid)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/docs", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should redirect to login
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Errorf("expected redirect for expired session, got %d", rr.Code)
	}
}

// --- CSRF integration tests ---

func TestCSRF_PostWithoutTokenRejected(t *testing.T) {
	router, _ := newTestServer(t)

	form := url.Values{"email": {"test@test.com"}, "password": {"test"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
		t.Errorf("expected 400 or 403 for missing CSRF, got %d", rr.Code)
	}
}

func TestCSRF_PostWithValidTokenAccepted(t *testing.T) {
	router, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	csrfRe := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
	matches := csrfRe.FindStringSubmatch(rr.Body.String())
	if len(matches) < 2 {
		t.Fatal("no CSRF token found")
	}
	cookies := rr.Result().Cookies()

	form := url.Values{"email": {"nobody@test.com"}, "password": {"wrong"}, "csrf_token": {matches[1]}}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusBadRequest || rr.Code == http.StatusForbidden {
		t.Errorf("CSRF should have been accepted, got %d", rr.Code)
	}
}

func TestCSRF_WrongTokenRejected(t *testing.T) {
	router, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	cookies := rr.Result().Cookies()

	form := url.Values{"email": {"test@test.com"}, "password": {"test"}, "csrf_token": {"completely-wrong"}}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
		t.Errorf("expected 400 or 403 for wrong CSRF token, got %d", rr.Code)
	}
}
