package auth

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	userContextKey     contextKey = "user"
	tenantContextKey   contextKey = "tenant"
	membershipCtxKey   contextKey = "membership"
)

type User struct {
	ID    uuid.UUID
	Email string
	Name  string
}

type Membership struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	UserID   uuid.UUID
	Role     string
}

type Tenant struct {
	ID   uuid.UUID
	Name string
}

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}

func ContextWithTenant(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenant)
}

func TenantFromContext(ctx context.Context) *Tenant {
	tenant, _ := ctx.Value(tenantContextKey).(*Tenant)
	return tenant
}

func ContextWithMembership(ctx context.Context, membership *Membership) context.Context {
	return context.WithValue(ctx, membershipCtxKey, membership)
}

func MembershipFromContext(ctx context.Context) *Membership {
	membership, _ := ctx.Value(membershipCtxKey).(*Membership)
	return membership
}

// Role checks
func (m *Membership) IsAdmin() bool {
	return m != nil && m.Role == "admin"
}

func (m *Membership) IsEditor() bool {
	return m != nil && (m.Role == "admin" || m.Role == "editor")
}

func (m *Membership) IsReader() bool {
	return m != nil // Any role can read
}
