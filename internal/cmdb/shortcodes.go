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
func (r *Repository) RenderShortcodes(ctx context.Context, tenantID uuid.UUID, rendered string) string {
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
			s, err := r.GetSystem(ctx, tenantID, scID)
			if err != nil {
				return warn("system", scID.String(), "not found")
			}
			return renderSystemBlock(s)
		case "vendor":
			v, err := r.GetVendor(ctx, tenantID, scID)
			if err != nil {
				return warn("vendor", scID.String(), "not found")
			}
			return renderVendorBlock(v)
		case "contact":
			c, err := r.GetContact(ctx, tenantID, scID)
			if err != nil {
				return warn("contact", scID.String(), "not found")
			}
			return renderContactBlock(c)
		case "circuit":
			c, err := r.GetCircuit(ctx, tenantID, scID)
			if err != nil {
				return warn("circuit", scID.String(), "not found")
			}
			return renderCircuitBlock(c)
		default:
			return match
		}
	})
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
