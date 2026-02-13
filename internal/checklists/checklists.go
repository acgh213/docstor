package checklists

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("checklist not found")

// Checklist is a reusable checklist definition.
type Checklist struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	CreatedBy   uuid.UUID
	CreatedAt   time.Time

	// Joined
	Items     []Item
	Author    *UserInfo
	ItemCount int
}

type Item struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ChecklistID uuid.UUID
	Position    int
	Text        string
}

// Instance is a started checklist, optionally linked to a document.
type Instance struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ChecklistID uuid.UUID
	LinkedType  *string
	LinkedID    *uuid.UUID
	Status      string // "in_progress" or "completed"
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	CompletedAt *time.Time

	// Joined
	Checklist     *Checklist
	Author        *UserInfo
	Items         []InstanceItem
	CompletedCount int
	TotalCount     int
	LinkedTitle    string // doc title when linked
}

type InstanceItem struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	InstanceID   uuid.UUID
	ItemID       uuid.UUID
	Done         bool
	DoneByUserID *uuid.UUID
	DoneAt       *time.Time
	Note         string

	// Joined from checklist_items
	Position int
	Text     string
	DoneBy   *UserInfo
}

type UserInfo struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// --- Checklist CRUD ---

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID) ([]Checklist, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.tenant_id, c.name, c.description, c.created_by, c.created_at,
		       u.id, u.name, u.email,
		       (SELECT COUNT(*) FROM checklist_items ci WHERE ci.checklist_id = c.id)
		FROM checklists c
		JOIN users u ON c.created_by = u.id
		WHERE c.tenant_id = $1
		ORDER BY c.name ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query checklists: %w", err)
	}
	defer rows.Close()

	var checklists []Checklist
	for rows.Next() {
		var cl Checklist
		var desc *string
		var author UserInfo
		if err := rows.Scan(
			&cl.ID, &cl.TenantID, &cl.Name, &desc, &cl.CreatedBy, &cl.CreatedAt,
			&author.ID, &author.Name, &author.Email,
			&cl.ItemCount,
		); err != nil {
			return nil, fmt.Errorf("scan checklist: %w", err)
		}
		if desc != nil {
			cl.Description = *desc
		}
		cl.Author = &author
		checklists = append(checklists, cl)
	}
	return checklists, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, checklistID uuid.UUID) (*Checklist, error) {
	var cl Checklist
	var desc *string
	var author UserInfo
	err := r.db.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.name, c.description, c.created_by, c.created_at,
		       u.id, u.name, u.email
		FROM checklists c
		JOIN users u ON c.created_by = u.id
		WHERE c.tenant_id = $1 AND c.id = $2
	`, tenantID, checklistID).Scan(
		&cl.ID, &cl.TenantID, &cl.Name, &desc, &cl.CreatedBy, &cl.CreatedAt,
		&author.ID, &author.Name, &author.Email,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query checklist: %w", err)
	}
	if desc != nil {
		cl.Description = *desc
	}
	cl.Author = &author

	// Load items
	cl.Items, err = r.ListItems(ctx, tenantID, checklistID)
	if err != nil {
		return nil, err
	}
	cl.ItemCount = len(cl.Items)

	return &cl, nil
}

type CreateInput struct {
	TenantID    uuid.UUID
	Name        string
	Description string
	CreatedBy   uuid.UUID
	Items       []string // item texts in order
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Checklist, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var desc *string
	if input.Description != "" {
		desc = &input.Description
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO checklists (tenant_id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, input.TenantID, input.Name, desc, input.CreatedBy).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert checklist: %w", err)
	}

	for i, text := range input.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO checklist_items (tenant_id, checklist_id, position, text)
			VALUES ($1, $2, $3, $4)
		`, input.TenantID, id, i, text)
		if err != nil {
			return nil, fmt.Errorf("insert checklist item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.Get(ctx, input.TenantID, id)
}

type UpdateInput struct {
	Name        string
	Description string
	Items       []string // replaces all items
}

func (r *Repository) Update(ctx context.Context, tenantID, checklistID uuid.UUID, input UpdateInput) (*Checklist, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var desc *string
	if input.Description != "" {
		desc = &input.Description
	}

	result, err := tx.Exec(ctx, `
		UPDATE checklists SET name = $3, description = $4 WHERE tenant_id = $1 AND id = $2
	`, tenantID, checklistID, input.Name, desc)
	if err != nil {
		return nil, fmt.Errorf("update checklist: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	// Replace items: delete old, insert new
	// First check no instances reference these items (if instances exist, we keep items)
	var instanceCount int
	_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM checklist_instances WHERE tenant_id = $1 AND checklist_id = $2`,
		tenantID, checklistID).Scan(&instanceCount)

	if instanceCount == 0 {
		// Safe to replace items
		_, _ = tx.Exec(ctx, `DELETE FROM checklist_items WHERE tenant_id = $1 AND checklist_id = $2`,
			tenantID, checklistID)
		for i, text := range input.Items {
			_, err = tx.Exec(ctx, `
				INSERT INTO checklist_items (tenant_id, checklist_id, position, text)
				VALUES ($1, $2, $3, $4)
			`, tenantID, checklistID, i, text)
			if err != nil {
				return nil, fmt.Errorf("insert checklist item: %w", err)
			}
		}
	}
	// If instances exist, only update name/description (items are locked)

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.Get(ctx, tenantID, checklistID)
}

func (r *Repository) Delete(ctx context.Context, tenantID, checklistID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM checklists WHERE tenant_id = $1 AND id = $2`,
		tenantID, checklistID)
	if err != nil {
		return fmt.Errorf("delete checklist: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListItems(ctx context.Context, tenantID, checklistID uuid.UUID) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, checklist_id, position, text
		FROM checklist_items
		WHERE tenant_id = $1 AND checklist_id = $2
		ORDER BY position ASC
	`, tenantID, checklistID)
	if err != nil {
		return nil, fmt.Errorf("query checklist items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.TenantID, &it.ChecklistID, &it.Position, &it.Text); err != nil {
			return nil, fmt.Errorf("scan checklist item: %w", err)
		}
		items = append(items, it)
	}
	return items, nil
}

// --- Instance CRUD ---

type StartInput struct {
	TenantID    uuid.UUID
	ChecklistID uuid.UUID
	LinkedType  *string
	LinkedID    *uuid.UUID
	CreatedBy   uuid.UUID
}

func (r *Repository) StartInstance(ctx context.Context, input StartInput) (*Instance, error) {
	// Verify checklist exists and get items
	cl, err := r.Get(ctx, input.TenantID, input.ChecklistID)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO checklist_instances (tenant_id, checklist_id, linked_type, linked_id, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, input.TenantID, input.ChecklistID, input.LinkedType, input.LinkedID, input.CreatedBy).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert instance: %w", err)
	}

	// Create instance items for each checklist item
	for _, item := range cl.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO checklist_instance_items (tenant_id, instance_id, item_id)
			VALUES ($1, $2, $3)
		`, input.TenantID, id, item.ID)
		if err != nil {
			return nil, fmt.Errorf("insert instance item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.GetInstance(ctx, input.TenantID, id)
}

func (r *Repository) GetInstance(ctx context.Context, tenantID, instanceID uuid.UUID) (*Instance, error) {
	var inst Instance
	var author UserInfo
	err := r.db.QueryRow(ctx, `
		SELECT ci.id, ci.tenant_id, ci.checklist_id, ci.linked_type, ci.linked_id,
		       ci.status, ci.created_by, ci.created_at, ci.completed_at,
		       u.id, u.name, u.email
		FROM checklist_instances ci
		JOIN users u ON ci.created_by = u.id
		WHERE ci.tenant_id = $1 AND ci.id = $2
	`, tenantID, instanceID).Scan(
		&inst.ID, &inst.TenantID, &inst.ChecklistID, &inst.LinkedType, &inst.LinkedID,
		&inst.Status, &inst.CreatedBy, &inst.CreatedAt, &inst.CompletedAt,
		&author.ID, &author.Name, &author.Email,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query instance: %w", err)
	}
	inst.Author = &author

	// Load checklist name
	cl, err := r.Get(ctx, tenantID, inst.ChecklistID)
	if err == nil {
		inst.Checklist = cl
	}

	// Load instance items with checklist item text
	inst.Items, err = r.ListInstanceItems(ctx, tenantID, instanceID)
	if err != nil {
		return nil, err
	}

	inst.TotalCount = len(inst.Items)
	for _, it := range inst.Items {
		if it.Done {
			inst.CompletedCount++
		}
	}

	// Load linked doc title
	if inst.LinkedType != nil && *inst.LinkedType == "document" && inst.LinkedID != nil {
		_ = r.db.QueryRow(ctx, `SELECT title FROM documents WHERE tenant_id = $1 AND id = $2`,
			tenantID, *inst.LinkedID).Scan(&inst.LinkedTitle)
	}

	return &inst, nil
}

func (r *Repository) ListInstances(ctx context.Context, tenantID uuid.UUID, status string) ([]Instance, error) {
	query := `
		SELECT ci.id, ci.tenant_id, ci.checklist_id, ci.linked_type, ci.linked_id,
		       ci.status, ci.created_by, ci.created_at, ci.completed_at,
		       u.id, u.name, u.email,
		       c.name,
		       (SELECT COUNT(*) FROM checklist_instance_items cii WHERE cii.instance_id = ci.id) AS total,
		       (SELECT COUNT(*) FROM checklist_instance_items cii WHERE cii.instance_id = ci.id AND cii.done = TRUE) AS completed,
		       d.title
		FROM checklist_instances ci
		JOIN users u ON ci.created_by = u.id
		JOIN checklists c ON ci.checklist_id = c.id
		LEFT JOIN documents d ON ci.linked_type = 'document' AND ci.linked_id = d.id AND d.tenant_id = ci.tenant_id
		WHERE ci.tenant_id = $1
	`
	args := []any{tenantID}
	if status != "" {
		query += " AND ci.status = $2"
		args = append(args, status)
	}
	query += " ORDER BY ci.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query instances: %w", err)
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var inst Instance
		var author UserInfo
		var checklistName string
		var linkedTitle *string
		if err := rows.Scan(
			&inst.ID, &inst.TenantID, &inst.ChecklistID, &inst.LinkedType, &inst.LinkedID,
			&inst.Status, &inst.CreatedBy, &inst.CreatedAt, &inst.CompletedAt,
			&author.ID, &author.Name, &author.Email,
			&checklistName,
			&inst.TotalCount, &inst.CompletedCount,
			&linkedTitle,
		); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		inst.Author = &author
		inst.Checklist = &Checklist{ID: inst.ChecklistID, Name: checklistName}
		if linkedTitle != nil {
			inst.LinkedTitle = *linkedTitle
		}

		instances = append(instances, inst)
	}
	return instances, nil
}

func (r *Repository) ListInstancesForDoc(ctx context.Context, tenantID, docID uuid.UUID) ([]Instance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ci.id, ci.tenant_id, ci.checklist_id, ci.linked_type, ci.linked_id,
		       ci.status, ci.created_by, ci.created_at, ci.completed_at,
		       u.id, u.name, u.email,
		       c.name,
		       (SELECT COUNT(*) FROM checklist_instance_items cii WHERE cii.instance_id = ci.id) AS total,
		       (SELECT COUNT(*) FROM checklist_instance_items cii WHERE cii.instance_id = ci.id AND cii.done = TRUE) AS completed
		FROM checklist_instances ci
		JOIN users u ON ci.created_by = u.id
		JOIN checklists c ON ci.checklist_id = c.id
		WHERE ci.tenant_id = $1 AND ci.linked_type = 'document' AND ci.linked_id = $2
		ORDER BY ci.created_at DESC
	`, tenantID, docID)
	if err != nil {
		return nil, fmt.Errorf("query instances for doc: %w", err)
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var inst Instance
		var author UserInfo
		var checklistName string
		if err := rows.Scan(
			&inst.ID, &inst.TenantID, &inst.ChecklistID, &inst.LinkedType, &inst.LinkedID,
			&inst.Status, &inst.CreatedBy, &inst.CreatedAt, &inst.CompletedAt,
			&author.ID, &author.Name, &author.Email,
			&checklistName,
			&inst.TotalCount, &inst.CompletedCount,
		); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		inst.Author = &author
		inst.Checklist = &Checklist{ID: inst.ChecklistID, Name: checklistName}
		instances = append(instances, inst)
	}
	return instances, nil
}

func (r *Repository) ListInstanceItems(ctx context.Context, tenantID, instanceID uuid.UUID) ([]InstanceItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cii.id, cii.tenant_id, cii.instance_id, cii.item_id,
		       cii.done, cii.done_by_user_id, cii.done_at, cii.note,
		       ci.position, ci.text,
		       u.id, u.name, u.email
		FROM checklist_instance_items cii
		JOIN checklist_items ci ON cii.item_id = ci.id
		LEFT JOIN users u ON cii.done_by_user_id = u.id
		WHERE cii.tenant_id = $1 AND cii.instance_id = $2
		ORDER BY ci.position ASC
	`, tenantID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("query instance items: %w", err)
	}
	defer rows.Close()

	var items []InstanceItem
	for rows.Next() {
		var it InstanceItem
		var note *string
		var doneByID, doneByName, doneByEmail *string
		if err := rows.Scan(
			&it.ID, &it.TenantID, &it.InstanceID, &it.ItemID,
			&it.Done, &it.DoneByUserID, &it.DoneAt, &note,
			&it.Position, &it.Text,
			&doneByID, &doneByName, &doneByEmail,
		); err != nil {
			return nil, fmt.Errorf("scan instance item: %w", err)
		}
		if note != nil {
			it.Note = *note
		}
		if doneByID != nil {
			uid, _ := uuid.Parse(*doneByID)
			it.DoneBy = &UserInfo{ID: uid, Name: *doneByName, Email: *doneByEmail}
		}
		items = append(items, it)
	}
	return items, nil
}

// ToggleItem checks or unchecks an instance item. Returns the updated item.
func (r *Repository) ToggleItem(ctx context.Context, tenantID, instanceID, itemID, userID uuid.UUID) (*InstanceItem, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock instance row to verify tenant scoping
	var instTenantID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT tenant_id FROM checklist_instances WHERE id = $1 FOR UPDATE`, instanceID).Scan(&instTenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query instance: %w", err)
	}
	if instTenantID != tenantID {
		return nil, ErrNotFound
	}

	// Get current state
	var currentDone bool
	var iitemID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id, done FROM checklist_instance_items
		WHERE tenant_id = $1 AND instance_id = $2 AND item_id = $3
	`, tenantID, instanceID, itemID).Scan(&iitemID, &currentDone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query instance item: %w", err)
	}

	newDone := !currentDone
	if newDone {
		_, err = tx.Exec(ctx, `
			UPDATE checklist_instance_items
			SET done = TRUE, done_by_user_id = $3, done_at = NOW()
			WHERE id = $1 AND tenant_id = $2
		`, iitemID, tenantID, userID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE checklist_instance_items
			SET done = FALSE, done_by_user_id = NULL, done_at = NULL
			WHERE id = $1 AND tenant_id = $2
		`, iitemID, tenantID)
	}
	if err != nil {
		return nil, fmt.Errorf("toggle item: %w", err)
	}

	// Check if all items are done -> mark instance completed
	var remaining int
	_ = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM checklist_instance_items
		WHERE instance_id = $1 AND done = FALSE
	`, instanceID).Scan(&remaining)

	if remaining == 0 {
		_, _ = tx.Exec(ctx, `UPDATE checklist_instances SET status = 'completed', completed_at = NOW() WHERE id = $1`, instanceID)
	} else {
		_, _ = tx.Exec(ctx, `UPDATE checklist_instances SET status = 'in_progress', completed_at = NULL WHERE id = $1`, instanceID)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Return updated item
	var it InstanceItem
	var noteStr *string
	_ = r.db.QueryRow(ctx, `
		SELECT cii.id, cii.tenant_id, cii.instance_id, cii.item_id,
		       cii.done, cii.done_by_user_id, cii.done_at, cii.note,
		       ci.position, ci.text
		FROM checklist_instance_items cii
		JOIN checklist_items ci ON cii.item_id = ci.id
		WHERE cii.id = $1
	`, iitemID).Scan(
		&it.ID, &it.TenantID, &it.InstanceID, &it.ItemID,
		&it.Done, &it.DoneByUserID, &it.DoneAt, &noteStr,
		&it.Position, &it.Text,
	)
	if noteStr != nil {
		it.Note = *noteStr
	}

	return &it, nil
}

func (r *Repository) DeleteInstance(ctx context.Context, tenantID, instanceID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM checklist_instances WHERE tenant_id = $1 AND id = $2`,
		tenantID, instanceID)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
