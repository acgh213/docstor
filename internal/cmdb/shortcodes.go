package cmdb

import (
	"context"
	"fmt"
	"html"
	"regexp"

	"github.com/google/uuid"
)

var shortcodeRe = regexp.MustCompile(`\{\{(system|vendor|contact|circuit):([0-9a-fA-F-]{36})\}\}`)

// RenderShortcodes replaces {{type:uuid}} shortcodes in rendered HTML with live blocks.
// Uses batch loading to avoid N+1 queries.
func (r *Repository) RenderShortcodes(ctx context.Context, tenantID uuid.UUID, rendered string) string {
	// First pass: collect all referenced UUIDs by type
	matches := shortcodeRe.FindAllStringSubmatch(rendered, -1)
	if len(matches) == 0 {
		return rendered
	}

	systemIDs := make(map[uuid.UUID]bool)
	vendorIDs := make(map[uuid.UUID]bool)
	contactIDs := make(map[uuid.UUID]bool)
	circuitIDs := make(map[uuid.UUID]bool)

	for _, m := range matches {
		if len(m) != 3 {
			continue
		}
		id, err := uuid.Parse(m[2])
		if err != nil {
			continue
		}
		switch m[1] {
		case "system":
			systemIDs[id] = true
		case "vendor":
			vendorIDs[id] = true
		case "contact":
			contactIDs[id] = true
		case "circuit":
			circuitIDs[id] = true
		}
	}

	// Batch load all entities
	systems := r.batchLoadSystems(ctx, tenantID, systemIDs)
	vendors := r.batchLoadVendors(ctx, tenantID, vendorIDs)
	contacts := r.batchLoadContacts(ctx, tenantID, contactIDs)
	circuits := r.batchLoadCircuits(ctx, tenantID, circuitIDs)

	// Second pass: replace shortcodes
	return shortcodeRe.ReplaceAllStringFunc(rendered, func(match string) string {
		parts := shortcodeRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		scType := parts[1]
		scID, err := uuid.Parse(parts[2])
		if err != nil {
			return warn(scType, parts[2], "invalid UUID")
		}

		switch scType {
		case "system":
			if s, ok := systems[scID]; ok {
				return renderSystemBlock(&s)
			}
			return warn("system", scID.String(), "not found")
		case "vendor":
			if v, ok := vendors[scID]; ok {
				return renderVendorBlock(&v)
			}
			return warn("vendor", scID.String(), "not found")
		case "contact":
			if c, ok := contacts[scID]; ok {
				return renderContactBlock(&c)
			}
			return warn("contact", scID.String(), "not found")
		case "circuit":
			if c, ok := circuits[scID]; ok {
				return renderCircuitBlock(&c)
			}
			return warn("circuit", scID.String(), "not found")
		default:
			return match
		}
	})
}

func (r *Repository) batchLoadSystems(ctx context.Context, tenantID uuid.UUID, ids map[uuid.UUID]bool) map[uuid.UUID]System {
	result := make(map[uuid.UUID]System, len(ids))
	if len(ids) == 0 {
		return result
	}
	idSlice := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		idSlice = append(idSlice, id)
	}
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, client_id, site_id, system_type, name, fqdn, ip, os, environment, notes, owner_user_id, created_at, updated_at FROM systems WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, idSlice)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var s System
		var fqdn, ip, osVal, notes *string
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ClientID, &s.SiteID, &s.SystemType, &s.Name, &fqdn, &ip, &osVal, &s.Environment, &notes, &s.OwnerUserID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			continue
		}
		s.FQDN = ns(fqdn)
		s.IP = ns(ip)
		s.OS = ns(osVal)
		s.Notes = ns(notes)
		result[s.ID] = s
	}
	return result
}

func (r *Repository) batchLoadVendors(ctx context.Context, tenantID uuid.UUID, ids map[uuid.UUID]bool) map[uuid.UUID]Vendor {
	result := make(map[uuid.UUID]Vendor, len(ids))
	if len(ids) == 0 {
		return result
	}
	idSlice := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		idSlice = append(idSlice, id)
	}
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, client_id, name, vendor_type, phone, email, portal_url, escalation_notes, notes, created_at FROM vendors WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, idSlice)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var v Vendor
		var phone, email, portalURL, escalationNotes, notes *string
		if err := rows.Scan(&v.ID, &v.TenantID, &v.ClientID, &v.Name, &v.VendorType, &phone, &email, &portalURL, &escalationNotes, &notes, &v.CreatedAt); err != nil {
			continue
		}
		v.Phone = ns(phone)
		v.Email = ns(email)
		v.PortalURL = ns(portalURL)
		v.EscalationNotes = ns(escalationNotes)
		v.Notes = ns(notes)
		result[v.ID] = v
	}
	return result
}

func (r *Repository) batchLoadContacts(ctx context.Context, tenantID uuid.UUID, ids map[uuid.UUID]bool) map[uuid.UUID]Contact {
	result := make(map[uuid.UUID]Contact, len(ids))
	if len(ids) == 0 {
		return result
	}
	idSlice := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		idSlice = append(idSlice, id)
	}
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, client_id, site_id, name, role, phone, email, notes, created_at FROM contacts WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, idSlice)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var c Contact
		var role, phone, email, notes *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ClientID, &c.SiteID, &c.Name, &role, &phone, &email, &notes, &c.CreatedAt); err != nil {
			continue
		}
		c.Role = ns(role)
		c.Phone = ns(phone)
		c.Email = ns(email)
		c.Notes = ns(notes)
		result[c.ID] = c
	}
	return result
}

func (r *Repository) batchLoadCircuits(ctx context.Context, tenantID uuid.UUID, ids map[uuid.UUID]bool) map[uuid.UUID]Circuit {
	result := make(map[uuid.UUID]Circuit, len(ids))
	if len(ids) == 0 {
		return result
	}
	idSlice := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		idSlice = append(idSlice, id)
	}
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, client_id, site_id, provider, circuit_id, circuit_type, wan_ip, speed, notes, created_at, updated_at FROM circuits WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, idSlice)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var c Circuit
		var wanIP, speed, notes *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ClientID, &c.SiteID, &c.Provider, &c.CircuitID, &c.CircuitType, &wanIP, &speed, &notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.WanIP = ns(wanIP)
		c.Speed = ns(speed)
		c.Notes = ns(notes)
		result[c.ID] = c
	}
	return result
}

func warn(scType, id, reason string) string {
	return fmt.Sprintf(`<span class="shortcode-warning">⚠️ %s not found: %s</span>`, html.EscapeString(scType), html.EscapeString(id))
}

func renderSystemBlock(s *System) string {
	details := html.EscapeString(s.Name)
	if s.IP != "" {
		details += " · " + html.EscapeString(s.IP)
	}
	if s.FQDN != "" {
		details += " · " + html.EscapeString(s.FQDN)
	}
	return fmt.Sprintf(`<span class="shortcode-block shortcode-system" title="%s"><a href="/systems/%s">🖥 %s</a> <small class="badge">%s</small></span>`,
		html.EscapeString(s.Environment), s.ID.String(), details, html.EscapeString(s.Environment))
}

func renderVendorBlock(v *Vendor) string {
	info := html.EscapeString(v.Name)
	if v.Phone != "" {
		info += " · " + html.EscapeString(v.Phone)
	}
	portal := ""
	if v.PortalURL != "" {
		portal = fmt.Sprintf(` · <a href="%s" target="_blank" rel="noopener noreferrer">Portal</a>`, html.EscapeString(v.PortalURL))
	}
	return fmt.Sprintf(`<span class="shortcode-block shortcode-vendor"><a href="/vendors/%s">🏢 %s</a>%s</span>`,
		v.ID.String(), info, portal)
}

func renderContactBlock(c *Contact) string {
	info := html.EscapeString(c.Name)
	if c.Role != "" {
		info += " (" + html.EscapeString(c.Role) + ")"
	}
	if c.Phone != "" {
		info += " · " + html.EscapeString(c.Phone)
	}
	if c.Email != "" {
		info += " · " + html.EscapeString(c.Email)
	}
	return fmt.Sprintf(`<span class="shortcode-block shortcode-contact"><a href="/contacts/%s">👤 %s</a></span>`,
		c.ID.String(), info)
}

func renderCircuitBlock(c *Circuit) string {
	info := html.EscapeString(c.Provider) + " " + html.EscapeString(c.CircuitID)
	if c.WanIP != "" {
		info += " · WAN: " + html.EscapeString(c.WanIP)
	}
	if c.Speed != "" {
		info += " · " + html.EscapeString(c.Speed)
	}
	return fmt.Sprintf(`<span class="shortcode-block shortcode-circuit"><a href="/circuits/%s">🔌 %s</a></span>`,
		c.ID.String(), info)
}
