package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SessionDuration = 24 * time.Hour * 7 // 7 days
	TokenLength     = 32
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TenantID  uuid.UUID
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

type SessionManager struct {
	db *pgxpool.Pool
}

func NewSessionManager(db *pgxpool.Pool) *SessionManager {
	return &SessionManager{db: db}
}

func generateToken() (string, string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(bytes)
	hash := hashToken(token)
	return token, hash, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (sm *SessionManager) Create(ctx context.Context, userID, tenantID uuid.UUID, ip, userAgent string) (string, error) {
	token, tokenHash, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	expiresAt := time.Now().Add(SessionDuration)

	_, err = sm.db.Exec(ctx, `
		INSERT INTO sessions (user_id, tenant_id, token_hash, expires_at, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, tenantID, tokenHash, expiresAt, ip, userAgent)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return token, nil
}

func (sm *SessionManager) Validate(ctx context.Context, token string) (*Session, error) {
	tokenHash := hashToken(token)

	var s Session
	err := sm.db.QueryRow(ctx, `
		SELECT id, user_id, tenant_id, token_hash, created_at, expires_at, ip, user_agent
		FROM sessions
		WHERE token_hash = $1
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TenantID, &s.TokenHash, &s.CreatedAt, &s.ExpiresAt, &s.IP, &s.UserAgent)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	if time.Now().After(s.ExpiresAt) {
		_ = sm.Delete(ctx, token)
		return nil, ErrSessionExpired
	}

	return &s, nil
}

func (sm *SessionManager) Delete(ctx context.Context, token string) error {
	tokenHash := hashToken(token)
	_, err := sm.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (sm *SessionManager) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := sm.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

func (sm *SessionManager) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := sm.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
