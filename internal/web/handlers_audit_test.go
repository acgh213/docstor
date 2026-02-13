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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/docs"
	"github.com/exedev/docstor/internal/testutil"
)

func hashForTest(password string) (string, error) {
	return auth.HashPassword(password)
}

// queryLastAuditAction returns the most recent audit action string for the given tenant.
func queryLastAuditAction(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) string {
	t.Helper()
	var action string
	err := pool.QueryRow(context.Background(),
		`SELECT action FROM audit_log WHERE tenant_id = $1 ORDER BY at DESC LIMIT 1`, tenantID).Scan(&action)
	if err != nil {
		t.Fatalf("query last audit action: %v", err)
	}
	return action
}

// countAuditActions returns how many audit rows match a given action for the tenant.
func countAuditActions(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, action string) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = $2`, tenantID, action).Scan(&count)
	if err != nil {
		t.Fatalf("count audit actions: %v", err)
	}
	return count
}

// ---------------------------------------------------------------------------
// A-2: Audit action integration tests
// ---------------------------------------------------------------------------

func TestAudit_DocCreate(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditDocCreate", "ed@audit.com", "editor")

	form := url.Values{
		"path":    {fmt.Sprintf("audit-create-%s", uuid.New().String()[:8])},
		"title":   {"Audit Create Test"},
		"body":    {"test body"},
		"message": {"init"},
	}
	rr := doRequest(t, router, "POST", "/docs/new", form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /docs/new: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionDocCreate {
		t.Errorf("audit action = %q, want %q", action, audit.ActionDocCreate)
	}
}

func TestAudit_DocEdit(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditDocEdit", "ed@audit-edit.com", "editor")

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("audit-edit-%s", uuid.New().String()[:8]),
		Title:     "Audit Edit",
		CreatedBy: editor.UserID,
		Body:      "original",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	form := url.Values{
		"body":             {"edited body"},
		"message":          {"edit commit"},
		"base_revision_id": {doc.CurrentRevisionID.String()},
	}
	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/save", form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST save: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionDocEdit {
		t.Errorf("audit action = %q, want %q", action, audit.ActionDocEdit)
	}
}

func TestAudit_DocRename(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditDocRename", "ed@audit-rename.com", "editor")

	repo := docs.NewRepository(env.Pool)
	oldPath := fmt.Sprintf("audit-rename-old-%s", uuid.New().String()[:8])
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      oldPath,
		Title:     "Rename Me",
		CreatedBy: editor.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	newPath := fmt.Sprintf("audit-rename-new-%s", uuid.New().String()[:8])
	form := url.Values{
		"new_path":  {newPath},
		"new_title": {"Renamed Doc"},
	}
	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/rename", form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST rename: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionDocMove {
		t.Errorf("audit action = %q, want %q", action, audit.ActionDocMove)
	}
}

func TestAudit_DocDelete(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditDocDelete", "ed@audit-del.com", "editor")

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("audit-delete-%s", uuid.New().String()[:8]),
		Title:     "Delete Me",
		CreatedBy: editor.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/delete", nil, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST delete: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionDocDelete {
		t.Errorf("audit action = %q, want %q", action, audit.ActionDocDelete)
	}
}

func TestAudit_DocRevert(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditDocRevert", "ed@audit-revert.com", "editor")

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("audit-revert-%s", uuid.New().String()[:8]),
		Title:     "Revert Me",
		CreatedBy: editor.UserID,
		Body:      "v1",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	// Create a second revision so we can revert to the first
	_, err = repo.Update(context.Background(), editor.TenantID, doc.ID, docs.UpdateInput{
		Body:           "v2",
		Message:        "edit",
		UpdatedBy:      editor.UserID,
		BaseRevisionID: *doc.CurrentRevisionID,
	})
	if err != nil {
		t.Fatalf("update doc: %v", err)
	}

	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/revert/"+doc.CurrentRevisionID.String(), nil, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST revert: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionDocRevert {
		t.Errorf("audit action = %q, want %q", action, audit.ActionDocRevert)
	}
}

func TestAudit_RunbookVerify(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditRunbookVerify", "ed@audit-verify.com", "editor")

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("audit-runbook-%s", uuid.New().String()[:8]),
		Title:     "Verify Me",
		DocType:   "runbook",
		CreatedBy: editor.UserID,
		Body:      "runbook steps",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	// Ensure runbook status exists
	_, err = env.Pool.Exec(context.Background(),
		`INSERT INTO runbook_status (document_id, tenant_id, verification_interval_days)
		 VALUES ($1, $2, 90) ON CONFLICT DO NOTHING`, doc.ID, editor.TenantID)
	if err != nil {
		t.Fatalf("insert runbook status: %v", err)
	}

	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/verify", nil, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST verify: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionRunbookVerify {
		t.Errorf("audit action = %q, want %q", action, audit.ActionRunbookVerify)
	}
}

func TestAudit_ClientCreate(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditClientCreate", "ed@audit-client.com", "editor")

	form := url.Values{
		"name": {"Audit Client"},
		"code": {fmt.Sprintf("AC-%s", uuid.New().String()[:8])},
	}
	rr := doRequest(t, router, "POST", "/clients/", form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /clients/: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionClientCreate {
		t.Errorf("audit action = %q, want %q", action, audit.ActionClientCreate)
	}
}

func TestAudit_ClientUpdate(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditClientUpdate", "ed@audit-clientupd.com", "editor")

	clientID := testutil.CreateClient(t, env.Pool, editor.TenantID, "Audit Client Upd", fmt.Sprintf("ACU-%s", uuid.New().String()[:8]))

	form := url.Values{
		"name":  {"Updated Client"},
		"code":  {fmt.Sprintf("UCL-%s", uuid.New().String()[:8])},
		"notes": {"updated"},
	}
	rr := doRequest(t, router, "POST", "/clients/"+clientID.String(), form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /clients/{id}: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionClientUpdate {
		t.Errorf("audit action = %q, want %q", action, audit.ActionClientUpdate)
	}
}

func TestAudit_MembershipAdd(t *testing.T) {
	router, env := newTestServer(t)
	admin := testutil.QuickFixture(t, env.Pool, "AuditMemberAdd", "admin@audit-member.com", "admin")

	form := url.Values{
		"email":    {fmt.Sprintf("%s.newuser@audit.com", uuid.New().String()[:8])},
		"name":     {"New User"},
		"password": {"password1234"},
		"role":     {"reader"},
	}
	rr := doRequest(t, router, "POST", "/admin/users", form, admin.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /admin/users: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, admin.TenantID)
	if action != audit.ActionMembershipAdd {
		t.Errorf("audit action = %q, want %q", action, audit.ActionMembershipAdd)
	}
}

func TestAudit_MembershipEdit(t *testing.T) {
	router, env := newTestServer(t)
	admin := testutil.QuickFixture(t, env.Pool, "AuditMemberEdit", "admin@audit-memedit.com", "admin")

	// Create a user to edit
	userID := testutil.CreateUser(t, env.Pool, "toedit@audit.com", "Edit Me", "password123")
	testutil.CreateMembership(t, env.Pool, admin.TenantID, userID, "reader")

	form := url.Values{
		"name":     {"Updated Name"},
		"role":     {"editor"},
	}
	rr := doRequest(t, router, "POST", "/admin/users/"+userID.String(), form, admin.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /admin/users/{id}: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, admin.TenantID)
	if action != audit.ActionMembershipEdit {
		t.Errorf("audit action = %q, want %q", action, audit.ActionMembershipEdit)
	}
}

func TestAudit_MembershipDelete(t *testing.T) {
	router, env := newTestServer(t)
	admin := testutil.QuickFixture(t, env.Pool, "AuditMemberDel", "admin@audit-memdel.com", "admin")

	// Create a user to delete
	userID := testutil.CreateUser(t, env.Pool, "todelete@audit.com", "Delete Me", "password123")
	testutil.CreateMembership(t, env.Pool, admin.TenantID, userID, "reader")

	rr := doRequest(t, router, "POST", "/admin/users/"+userID.String()+"/delete", nil, admin.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /admin/users/{id}/delete: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, admin.TenantID)
	if action != audit.ActionMembershipDel {
		t.Errorf("audit action = %q, want %q", action, audit.ActionMembershipDel)
	}
}

// doLoginRequest issues a POST /login with CSRF token obtained from GET /login
// (not from / which requires auth).
func doLoginRequest(t *testing.T, handler http.Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	// GET /login to get CSRF cookie + token
	getReq := httptest.NewRequest("GET", "/login", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	csrfCookies := getRR.Result().Cookies()
	re := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
	var csrfToken string
	if m := re.FindStringSubmatch(getRR.Body.String()); len(m) == 2 {
		csrfToken = m[1]
	}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	for _, c := range csrfCookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestAudit_LoginSuccess(t *testing.T) {
	router, env := newTestServer(t)
	tid := testutil.CreateTenant(t, env.Pool, "AuditLoginSuccess")
	email := fmt.Sprintf("%s.login@audit.com", uuid.New().String()[:8])
	var uid uuid.UUID
	{
		hash, _ := hashForTest("password123")
		err := env.Pool.QueryRow(context.Background(),
			`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
			email, "Login User", hash).Scan(&uid)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	testutil.CreateMembership(t, env.Pool, tid, uid, "editor")

	form := url.Values{
		"email":    {email},
		"password": {"password123"},
	}
	rr := doLoginRequest(t, router, form)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /login: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	action := queryLastAuditAction(t, env.Pool, tid)
	if action != audit.ActionLoginSuccess {
		t.Errorf("audit action = %q, want %q", action, audit.ActionLoginSuccess)
	}
}

func TestAudit_LoginFailure(t *testing.T) {
	router, env := newTestServer(t)
	tid := testutil.CreateTenant(t, env.Pool, "AuditLoginFail")
	email := fmt.Sprintf("%s.loginfail@audit.com", uuid.New().String()[:8])
	var uid uuid.UUID
	{
		hash, _ := hashForTest("password123")
		err := env.Pool.QueryRow(context.Background(),
			`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
			email, "Fail User", hash).Scan(&uid)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	testutil.CreateMembership(t, env.Pool, tid, uid, "reader")

	form := url.Values{
		"email":    {email},
		"password": {"wrongpassword"},
	}
	_ = doLoginRequest(t, router, form)

	action := queryLastAuditAction(t, env.Pool, tid)
	if action != audit.ActionLoginFailed {
		t.Errorf("audit action = %q, want %q", action, audit.ActionLoginFailed)
	}
}

func TestAudit_Logout(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "AuditLogout", "ed@audit-logout.com", "editor")

	rr := doRequest(t, router, "POST", "/logout", nil, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /logout: got %d, want 303", rr.Code)
	}

	action := queryLastAuditAction(t, env.Pool, editor.TenantID)
	if action != audit.ActionLogout {
		t.Errorf("audit action = %q, want %q", action, audit.ActionLogout)
	}
}

// ---------------------------------------------------------------------------
// A-3: Doc rename/move/delete tests
// ---------------------------------------------------------------------------

func TestDocRename_HappyPath(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "RenameHappy", "ed@rename.com", "editor")

	repo := docs.NewRepository(env.Pool)
	oldPath := fmt.Sprintf("rename-happy-%s", uuid.New().String()[:8])
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      oldPath,
		Title:     "Old Title",
		CreatedBy: editor.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	newPath := fmt.Sprintf("rename-happy-new-%s", uuid.New().String()[:8])
	form := url.Values{
		"new_path":  {newPath},
		"new_title": {"New Title"},
	}
	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/rename", form, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST rename: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify path updated
	updated, err := repo.GetByID(context.Background(), editor.TenantID, doc.ID)
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}
	if updated.Path != newPath {
		t.Errorf("path = %q, want %q", updated.Path, newPath)
	}
	if updated.Title != "New Title" {
		t.Errorf("title = %q, want %q", updated.Title, "New Title")
	}
}

func TestDocDelete_HappyPath(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "DeleteHappy", "ed@delete.com", "editor")

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("delete-happy-%s", uuid.New().String()[:8]),
		Title:     "Delete Me",
		CreatedBy: editor.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/delete", nil, editor.SessionToken)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST delete: got %d, want 303. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify doc is gone
	_, err = repo.GetByID(context.Background(), editor.TenantID, doc.ID)
	if err == nil {
		t.Error("expected error getting deleted doc, got nil")
	}
}

func TestDocRename_ReaderForbidden(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "RenameRoleDeny", "ed@renamerole.com", "editor")
	readerUID := testutil.CreateUser(t, env.Pool, "reader@renamerole.com", "Reader", "password123")
	testutil.CreateMembership(t, env.Pool, editor.TenantID, readerUID, "reader")
	readerToken := testutil.CreateSession(t, env.Pool, readerUID, editor.TenantID)

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("rename-deny-%s", uuid.New().String()[:8]),
		Title:     "No Rename",
		CreatedBy: editor.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	form := url.Values{
		"new_path":  {"hacked-path"},
		"new_title": {"Hacked Title"},
	}
	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/rename", form, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST rename as reader: got %d, want 403", rr.Code)
	}
}

func TestDocDelete_ReaderForbidden(t *testing.T) {
	router, env := newTestServer(t)
	editor := testutil.QuickFixture(t, env.Pool, "DeleteRoleDeny", "ed@deleterole.com", "editor")
	readerUID := testutil.CreateUser(t, env.Pool, "reader@deleterole.com", "Reader", "password123")
	testutil.CreateMembership(t, env.Pool, editor.TenantID, readerUID, "reader")
	readerToken := testutil.CreateSession(t, env.Pool, readerUID, editor.TenantID)

	repo := docs.NewRepository(env.Pool)
	doc, err := repo.Create(context.Background(), docs.CreateInput{
		TenantID:  editor.TenantID,
		Path:      fmt.Sprintf("delete-deny-%s", uuid.New().String()[:8]),
		Title:     "No Delete",
		CreatedBy: editor.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	rr := doRequest(t, router, "POST", "/docs/id/"+doc.ID.String()+"/delete", nil, readerToken)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST delete as reader: got %d, want 403", rr.Code)
	}
}
