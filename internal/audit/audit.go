package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	db *pgxpool.Pool
}

func NewLogger(db *pgxpool.Pool) *Logger {
	return &Logger{db: db}
}

type Entry struct {
	TenantID    uuid.UUID
	ActorUserID *uuid.UUID
	Action      string
	TargetType  string
	TargetID    *uuid.UUID
	IP          string
	UserAgent   string
	Metadata    map[string]any
}

func (l *Logger) Log(ctx context.Context, e Entry) error {
	metadataJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = l.db.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, actor_user_id, action, target_type, target_id, ip, user_agent, metadata_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, e.TenantID, e.ActorUserID, e.Action, e.TargetType, e.TargetID, e.IP, e.UserAgent, metadataJSON)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

// Convenience method for logging without tenant context (e.g., login attempts)
func (l *Logger) LogGlobal(ctx context.Context, action, targetType string, metadata map[string]any, ip, userAgent string) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	// For global logs without tenant context, use a nil tenant_id approach
	// But our schema requires tenant_id, so we need to handle this differently
	// For login attempts, we can log after we determine the tenant
	_, err = l.db.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, actor_user_id, action, target_type, target_id, ip, user_agent, metadata_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, nil, nil, action, targetType, nil, ip, userAgent, metadataJSON)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

// Common action constants
const (
	ActionLoginSuccess   = "login.success"
	ActionLoginFailed    = "login.failed"
	ActionLogout         = "logout"
	ActionDocCreate      = "doc.create"
	ActionDocEdit        = "doc.edit"
	ActionDocDelete      = "doc.delete"
	ActionDocMove        = "doc.move"
	ActionDocRevert      = "doc.revert"
	ActionDocMetadata    = "doc.metadata"
	ActionRunbookVerify  = "runbook.verify"
	ActionClientCreate   = "client.create"
	ActionClientUpdate   = "client.update"
	ActionMembershipAdd  = "membership.add"
	ActionMembershipEdit = "membership.edit"
	ActionMembershipDel  = "membership.delete"

	// Templates
	ActionTemplateCreate = "template.create"
	ActionTemplateUpdate = "template.update"
	ActionTemplateDelete = "template.delete"

	// Checklists
	ActionChecklistCreate   = "checklist.create"
	ActionChecklistUpdate   = "checklist.update"
	ActionChecklistDelete   = "checklist.delete"
	ActionChecklistStart    = "checklist.instance.start"
	ActionChecklistToggle   = "checklist.instance.toggle"
	ActionChecklistComplete = "checklist.instance.complete"

	// CMDB
	ActionSystemCreate  = "system.create"
	ActionSystemUpdate  = "system.update"
	ActionSystemDelete  = "system.delete"
	ActionVendorCreate  = "vendor.create"
	ActionVendorUpdate  = "vendor.update"
	ActionVendorDelete  = "vendor.delete"
	ActionContactCreate = "contact.create"
	ActionContactUpdate = "contact.update"
	ActionContactDelete = "contact.delete"
	ActionCircuitCreate = "circuit.create"
	ActionCircuitUpdate = "circuit.update"
	ActionCircuitDelete = "circuit.delete"

	// Incidents
	ActionKnownIssueCreate = "known_issue.create"
	ActionKnownIssueUpdate = "known_issue.update"
	ActionKnownIssueDelete = "known_issue.delete"
	ActionIncidentCreate   = "incident.create"
	ActionIncidentUpdate   = "incident.update"
	ActionIncidentDelete   = "incident.delete"
	ActionIncidentEvent    = "incident.event"

	ActionSiteCreate = "site.create"
	ActionSiteUpdate = "site.update"
	ActionSiteDelete = "site.delete"
)

// Common target types
const (
	TargetUser       = "user"
	TargetTenant     = "tenant"
	TargetDocument   = "document"
	TargetRevision   = "revision"
	TargetClient     = "client"
	TargetMembership  = "membership"
	TargetTemplate    = "template"
	TargetChecklist   = "checklist"
	TargetCLInstance  = "checklist_instance"
	TargetSystem      = "system"
	TargetVendor      = "vendor"
	TargetContact     = "contact"
	TargetCircuit      = "circuit"
	TargetKnownIssue   = "known_issue"
	TargetIncident     = "incident"
	TargetIncidentEvent = "incident_event"
	TargetSite          = "site"
)
