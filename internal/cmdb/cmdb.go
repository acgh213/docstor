package cmdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

// --- Types ---

type System struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ClientID    *uuid.UUID
	SystemType  string
	Name        string
	FQDN        string
	IP          string
	OS          string
	Environment string
	Notes       string
	OwnerUserID *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Vendor struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ClientID        *uuid.UUID
	Name            string
	VendorType      string
	Phone           string
	Email           string
	PortalURL       string
	EscalationNotes string
	Notes           string
	CreatedAt       time.Time
}

type Contact struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ClientID  *uuid.UUID
	Name      string
	Role      string
	Phone     string
	Email     string
	Notes     string
	CreatedAt time.Time
}

type Circuit struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ClientID    *uuid.UUID
	Provider    string
	CircuitID   string
	CircuitType string
	WanIP       string
	Speed       string
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// --- Repository ---

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// helper to coalesce nullable strings
func ns(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// ===================== SYSTEMS =====================

func (r *Repository) ListSystems(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID) ([]System, error) {
	query := `SELECT id, tenant_id, client_id, system_type, name, fqdn, ip, os, environment, notes, owner_user_id, created_at, updated_at FROM systems WHERE tenant_id = $1`
	args := []any{tenantID}
	if clientID != nil {
		query += " AND client_id = $2"
		args = append(args, *clientID)
	}
	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query systems: %w", err)
	}
	defer rows.Close()

	var out []System
	for rows.Next() {
		var s System
		var fqdn, ip, os, notes *string
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ClientID, &s.SystemType, &s.Name, &fqdn, &ip, &os, &s.Environment, &notes, &s.OwnerUserID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan system: %w", err)
		}
		s.FQDN = ns(fqdn)
		s.IP = ns(ip)
		s.OS = ns(os)
		s.Notes = ns(notes)
		out = append(out, s)
	}
	return out, nil
}

func (r *Repository) GetSystem(ctx context.Context, tenantID, id uuid.UUID) (*System, error) {
	var s System
	var fqdn, ip, osVal, notes *string
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, client_id, system_type, name, fqdn, ip, os, environment, notes, owner_user_id, created_at, updated_at FROM systems WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&s.ID, &s.TenantID, &s.ClientID, &s.SystemType, &s.Name, &fqdn, &ip, &osVal, &s.Environment, &notes, &s.OwnerUserID, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get system: %w", err)
	}
	s.FQDN = ns(fqdn)
	s.IP = ns(ip)
	s.OS = ns(osVal)
	s.Notes = ns(notes)
	return &s, nil
}

type CreateSystemInput struct {
	TenantID    uuid.UUID
	ClientID    *uuid.UUID
	SystemType  string
	Name        string
	FQDN        string
	IP          string
	OS          string
	Environment string
	Notes       string
	OwnerUserID *uuid.UUID
}

func (r *Repository) CreateSystem(ctx context.Context, in CreateSystemInput) (*System, error) {
	var s System
	var fqdn, ip, osVal, notes *string
	if in.FQDN != "" {
		fqdn = &in.FQDN
	}
	if in.IP != "" {
		ip = &in.IP
	}
	if in.OS != "" {
		osVal = &in.OS
	}
	if in.Notes != "" {
		notes = &in.Notes
	}
	err := r.db.QueryRow(ctx, `INSERT INTO systems (tenant_id, client_id, system_type, name, fqdn, ip, os, environment, notes, owner_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, tenant_id, client_id, system_type, name, fqdn, ip, os, environment, notes, owner_user_id, created_at, updated_at`,
		in.TenantID, in.ClientID, in.SystemType, in.Name, fqdn, ip, osVal, in.Environment, notes, in.OwnerUserID,
	).Scan(&s.ID, &s.TenantID, &s.ClientID, &s.SystemType, &s.Name, &fqdn, &ip, &osVal, &s.Environment, &notes, &s.OwnerUserID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create system: %w", err)
	}
	s.FQDN = ns(fqdn)
	s.IP = ns(ip)
	s.OS = ns(osVal)
	s.Notes = ns(notes)
	return &s, nil
}

type UpdateSystemInput struct {
	ClientID    *uuid.UUID
	SystemType  string
	Name        string
	FQDN        string
	IP          string
	OS          string
	Environment string
	Notes       string
	OwnerUserID *uuid.UUID
}

func (r *Repository) UpdateSystem(ctx context.Context, tenantID, id uuid.UUID, in UpdateSystemInput) (*System, error) {
	var s System
	var fqdn, ip, osVal, notes *string
	if in.FQDN != "" {
		fqdn = &in.FQDN
	}
	if in.IP != "" {
		ip = &in.IP
	}
	if in.OS != "" {
		osVal = &in.OS
	}
	if in.Notes != "" {
		notes = &in.Notes
	}
	err := r.db.QueryRow(ctx, `UPDATE systems SET client_id=$3, system_type=$4, name=$5, fqdn=$6, ip=$7, os=$8, environment=$9, notes=$10, owner_user_id=$11, updated_at=now() WHERE tenant_id=$1 AND id=$2 RETURNING id, tenant_id, client_id, system_type, name, fqdn, ip, os, environment, notes, owner_user_id, created_at, updated_at`,
		tenantID, id, in.ClientID, in.SystemType, in.Name, fqdn, ip, osVal, in.Environment, notes, in.OwnerUserID,
	).Scan(&s.ID, &s.TenantID, &s.ClientID, &s.SystemType, &s.Name, &fqdn, &ip, &osVal, &s.Environment, &notes, &s.OwnerUserID, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update system: %w", err)
	}
	s.FQDN = ns(fqdn)
	s.IP = ns(ip)
	s.OS = ns(osVal)
	s.Notes = ns(notes)
	return &s, nil
}

func (r *Repository) DeleteSystem(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM systems WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete system: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ===================== VENDORS =====================

func (r *Repository) ListVendors(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID) ([]Vendor, error) {
	query := `SELECT id, tenant_id, client_id, name, vendor_type, phone, email, portal_url, escalation_notes, notes, created_at FROM vendors WHERE tenant_id = $1`
	args := []any{tenantID}
	if clientID != nil {
		query += " AND client_id = $2"
		args = append(args, *clientID)
	}
	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query vendors: %w", err)
	}
	defer rows.Close()

	var out []Vendor
	for rows.Next() {
		var v Vendor
		var phone, email, portalURL, escalationNotes, notes *string
		if err := rows.Scan(&v.ID, &v.TenantID, &v.ClientID, &v.Name, &v.VendorType, &phone, &email, &portalURL, &escalationNotes, &notes, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vendor: %w", err)
		}
		v.Phone = ns(phone)
		v.Email = ns(email)
		v.PortalURL = ns(portalURL)
		v.EscalationNotes = ns(escalationNotes)
		v.Notes = ns(notes)
		out = append(out, v)
	}
	return out, nil
}

func (r *Repository) GetVendor(ctx context.Context, tenantID, id uuid.UUID) (*Vendor, error) {
	var v Vendor
	var phone, email, portalURL, escalationNotes, notes *string
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, client_id, name, vendor_type, phone, email, portal_url, escalation_notes, notes, created_at FROM vendors WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&v.ID, &v.TenantID, &v.ClientID, &v.Name, &v.VendorType, &phone, &email, &portalURL, &escalationNotes, &notes, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vendor: %w", err)
	}
	v.Phone = ns(phone)
	v.Email = ns(email)
	v.PortalURL = ns(portalURL)
	v.EscalationNotes = ns(escalationNotes)
	v.Notes = ns(notes)
	return &v, nil
}

type CreateVendorInput struct {
	TenantID        uuid.UUID
	ClientID        *uuid.UUID
	Name            string
	VendorType      string
	Phone           string
	Email           string
	PortalURL       string
	EscalationNotes string
	Notes           string
}

func (r *Repository) CreateVendor(ctx context.Context, in CreateVendorInput) (*Vendor, error) {
	var v Vendor
	var phone, email, portalURL, escalationNotes, notes *string
	if in.Phone != "" { phone = &in.Phone }
	if in.Email != "" { email = &in.Email }
	if in.PortalURL != "" { portalURL = &in.PortalURL }
	if in.EscalationNotes != "" { escalationNotes = &in.EscalationNotes }
	if in.Notes != "" { notes = &in.Notes }
	err := r.db.QueryRow(ctx, `INSERT INTO vendors (tenant_id, client_id, name, vendor_type, phone, email, portal_url, escalation_notes, notes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, tenant_id, client_id, name, vendor_type, phone, email, portal_url, escalation_notes, notes, created_at`,
		in.TenantID, in.ClientID, in.Name, in.VendorType, phone, email, portalURL, escalationNotes, notes,
	).Scan(&v.ID, &v.TenantID, &v.ClientID, &v.Name, &v.VendorType, &phone, &email, &portalURL, &escalationNotes, &notes, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create vendor: %w", err)
	}
	v.Phone = ns(phone)
	v.Email = ns(email)
	v.PortalURL = ns(portalURL)
	v.EscalationNotes = ns(escalationNotes)
	v.Notes = ns(notes)
	return &v, nil
}

type UpdateVendorInput struct {
	ClientID        *uuid.UUID
	Name            string
	VendorType      string
	Phone           string
	Email           string
	PortalURL       string
	EscalationNotes string
	Notes           string
}

func (r *Repository) UpdateVendor(ctx context.Context, tenantID, id uuid.UUID, in UpdateVendorInput) (*Vendor, error) {
	var v Vendor
	var phone, email, portalURL, escalationNotes, notes *string
	if in.Phone != "" { phone = &in.Phone }
	if in.Email != "" { email = &in.Email }
	if in.PortalURL != "" { portalURL = &in.PortalURL }
	if in.EscalationNotes != "" { escalationNotes = &in.EscalationNotes }
	if in.Notes != "" { notes = &in.Notes }
	err := r.db.QueryRow(ctx, `UPDATE vendors SET client_id=$3, name=$4, vendor_type=$5, phone=$6, email=$7, portal_url=$8, escalation_notes=$9, notes=$10 WHERE tenant_id=$1 AND id=$2 RETURNING id, tenant_id, client_id, name, vendor_type, phone, email, portal_url, escalation_notes, notes, created_at`,
		tenantID, id, in.ClientID, in.Name, in.VendorType, phone, email, portalURL, escalationNotes, notes,
	).Scan(&v.ID, &v.TenantID, &v.ClientID, &v.Name, &v.VendorType, &phone, &email, &portalURL, &escalationNotes, &notes, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update vendor: %w", err)
	}
	v.Phone = ns(phone)
	v.Email = ns(email)
	v.PortalURL = ns(portalURL)
	v.EscalationNotes = ns(escalationNotes)
	v.Notes = ns(notes)
	return &v, nil
}

func (r *Repository) DeleteVendor(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM vendors WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete vendor: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ===================== CONTACTS =====================

func (r *Repository) ListContacts(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID) ([]Contact, error) {
	query := `SELECT id, tenant_id, client_id, name, role, phone, email, notes, created_at FROM contacts WHERE tenant_id = $1`
	args := []any{tenantID}
	if clientID != nil {
		query += " AND client_id = $2"
		args = append(args, *clientID)
	}
	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	var out []Contact
	for rows.Next() {
		var c Contact
		var role, phone, email, notes *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Name, &role, &phone, &email, &notes, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		c.Role = ns(role)
		c.Phone = ns(phone)
		c.Email = ns(email)
		c.Notes = ns(notes)
		out = append(out, c)
	}
	return out, nil
}

func (r *Repository) GetContact(ctx context.Context, tenantID, id uuid.UUID) (*Contact, error) {
	var c Contact
	var role, phone, email, notes *string
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, client_id, name, role, phone, email, notes, created_at FROM contacts WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Name, &role, &phone, &email, &notes, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	c.Role = ns(role)
	c.Phone = ns(phone)
	c.Email = ns(email)
	c.Notes = ns(notes)
	return &c, nil
}

type CreateContactInput struct {
	TenantID uuid.UUID
	ClientID *uuid.UUID
	Name     string
	Role     string
	Phone    string
	Email    string
	Notes    string
}

func (r *Repository) CreateContact(ctx context.Context, in CreateContactInput) (*Contact, error) {
	var c Contact
	var role, phone, email, notes *string
	if in.Role != "" { role = &in.Role }
	if in.Phone != "" { phone = &in.Phone }
	if in.Email != "" { email = &in.Email }
	if in.Notes != "" { notes = &in.Notes }
	err := r.db.QueryRow(ctx, `INSERT INTO contacts (tenant_id, client_id, name, role, phone, email, notes) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, tenant_id, client_id, name, role, phone, email, notes, created_at`,
		in.TenantID, in.ClientID, in.Name, role, phone, email, notes,
	).Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Name, &role, &phone, &email, &notes, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	c.Role = ns(role)
	c.Phone = ns(phone)
	c.Email = ns(email)
	c.Notes = ns(notes)
	return &c, nil
}

type UpdateContactInput struct {
	ClientID *uuid.UUID
	Name     string
	Role     string
	Phone    string
	Email    string
	Notes    string
}

func (r *Repository) UpdateContact(ctx context.Context, tenantID, id uuid.UUID, in UpdateContactInput) (*Contact, error) {
	var c Contact
	var role, phone, email, notes *string
	if in.Role != "" { role = &in.Role }
	if in.Phone != "" { phone = &in.Phone }
	if in.Email != "" { email = &in.Email }
	if in.Notes != "" { notes = &in.Notes }
	err := r.db.QueryRow(ctx, `UPDATE contacts SET client_id=$3, name=$4, role=$5, phone=$6, email=$7, notes=$8 WHERE tenant_id=$1 AND id=$2 RETURNING id, tenant_id, client_id, name, role, phone, email, notes, created_at`,
		tenantID, id, in.ClientID, in.Name, role, phone, email, notes,
	).Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Name, &role, &phone, &email, &notes, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	c.Role = ns(role)
	c.Phone = ns(phone)
	c.Email = ns(email)
	c.Notes = ns(notes)
	return &c, nil
}

func (r *Repository) DeleteContact(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM contacts WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ===================== CIRCUITS =====================

func (r *Repository) ListCircuits(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID) ([]Circuit, error) {
	query := `SELECT id, tenant_id, client_id, provider, circuit_id, circuit_type, wan_ip, speed, notes, created_at, updated_at FROM circuits WHERE tenant_id = $1`
	args := []any{tenantID}
	if clientID != nil {
		query += " AND client_id = $2"
		args = append(args, *clientID)
	}
	query += " ORDER BY provider ASC, circuit_id ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query circuits: %w", err)
	}
	defer rows.Close()

	var out []Circuit
	for rows.Next() {
		var c Circuit
		var wanIP, speed, notes *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Provider, &c.CircuitID, &c.CircuitType, &wanIP, &speed, &notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan circuit: %w", err)
		}
		c.WanIP = ns(wanIP)
		c.Speed = ns(speed)
		c.Notes = ns(notes)
		out = append(out, c)
	}
	return out, nil
}

func (r *Repository) GetCircuit(ctx context.Context, tenantID, id uuid.UUID) (*Circuit, error) {
	var c Circuit
	var wanIP, speed, notes *string
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, client_id, provider, circuit_id, circuit_type, wan_ip, speed, notes, created_at, updated_at FROM circuits WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Provider, &c.CircuitID, &c.CircuitType, &wanIP, &speed, &notes, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get circuit: %w", err)
	}
	c.WanIP = ns(wanIP)
	c.Speed = ns(speed)
	c.Notes = ns(notes)
	return &c, nil
}

type CreateCircuitInput struct {
	TenantID    uuid.UUID
	ClientID    *uuid.UUID
	Provider    string
	CircuitID   string
	CircuitType string
	WanIP       string
	Speed       string
	Notes       string
}

func (r *Repository) CreateCircuit(ctx context.Context, in CreateCircuitInput) (*Circuit, error) {
	var c Circuit
	var wanIP, speed, notes *string
	if in.WanIP != "" { wanIP = &in.WanIP }
	if in.Speed != "" { speed = &in.Speed }
	if in.Notes != "" { notes = &in.Notes }
	err := r.db.QueryRow(ctx, `INSERT INTO circuits (tenant_id, client_id, provider, circuit_id, circuit_type, wan_ip, speed, notes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, tenant_id, client_id, provider, circuit_id, circuit_type, wan_ip, speed, notes, created_at, updated_at`,
		in.TenantID, in.ClientID, in.Provider, in.CircuitID, in.CircuitType, wanIP, speed, notes,
	).Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Provider, &c.CircuitID, &c.CircuitType, &wanIP, &speed, &notes, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create circuit: %w", err)
	}
	c.WanIP = ns(wanIP)
	c.Speed = ns(speed)
	c.Notes = ns(notes)
	return &c, nil
}

type UpdateCircuitInput struct {
	ClientID    *uuid.UUID
	Provider    string
	CircuitID   string
	CircuitType string
	WanIP       string
	Speed       string
	Notes       string
}

func (r *Repository) UpdateCircuit(ctx context.Context, tenantID, id uuid.UUID, in UpdateCircuitInput) (*Circuit, error) {
	var c Circuit
	var wanIP, speed, notes *string
	if in.WanIP != "" { wanIP = &in.WanIP }
	if in.Speed != "" { speed = &in.Speed }
	if in.Notes != "" { notes = &in.Notes }
	err := r.db.QueryRow(ctx, `UPDATE circuits SET client_id=$3, provider=$4, circuit_id=$5, circuit_type=$6, wan_ip=$7, speed=$8, notes=$9, updated_at=now() WHERE tenant_id=$1 AND id=$2 RETURNING id, tenant_id, client_id, provider, circuit_id, circuit_type, wan_ip, speed, notes, created_at, updated_at`,
		tenantID, id, in.ClientID, in.Provider, in.CircuitID, in.CircuitType, wanIP, speed, notes,
	).Scan(&c.ID, &c.TenantID, &c.ClientID, &c.Provider, &c.CircuitID, &c.CircuitType, &wanIP, &speed, &notes, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update circuit: %w", err)
	}
	c.WanIP = ns(wanIP)
	c.Speed = ns(speed)
	c.Notes = ns(notes)
	return &c, nil
}

func (r *Repository) DeleteCircuit(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM circuits WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete circuit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
