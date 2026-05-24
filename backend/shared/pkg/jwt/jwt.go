package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the custom JWT claims structure embedded in every XynPOS token.
type Claims struct {
	gojwt.RegisteredClaims
	UserID      string   `json:"sub"`
	TenantID    string   `json:"tenant_id"`
	OutletID    string   `json:"outlet_id,omitempty"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Plan        string   `json:"plan"`
	TokenType   string   `json:"token_type"` // "access" | "refresh"
}

// Config holds JWT signing configuration.
type Config struct {
	AccessSecret  string        // HS256 secret (or PEM private key for RS256)
	RefreshSecret string        // HS256 secret for refresh tokens
	AccessExpiry  time.Duration // e.g. 15 * time.Minute
	RefreshExpiry time.Duration // e.g. 30 * 24 * time.Hour
	Issuer        string        // e.g. "xynpos.com"
}

// Manager handles JWT generation and validation.
type Manager struct {
	cfg Config
}

// New creates a new JWT Manager.
func New(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

// GenerateAccessToken creates a short-lived access token (HS256).
func (m *Manager) GenerateAccessToken(
	userID, tenantID, outletID, role, plan string,
	permissions []string,
) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    m.cfg.Issuer,
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(m.cfg.AccessExpiry)),
		},
		UserID:      userID,
		TenantID:    tenantID,
		OutletID:    outletID,
		Role:        role,
		Permissions: permissions,
		Plan:        plan,
		TokenType:   "access",
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(m.cfg.AccessSecret))
	if err != nil {
		return "", fmt.Errorf("jwt: sign access token: %w", err)
	}
	return signed, nil
}

// GenerateRefreshToken creates a long-lived refresh token (HS256).
// The token family allows detecting token reuse.
func (m *Manager) GenerateRefreshToken(userID, tenantID, tokenFamily string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    m.cfg.Issuer,
			Subject:   tokenFamily, // use family as subject for reuse detection
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(m.cfg.RefreshExpiry)),
		},
		UserID:    userID,
		TenantID:  tenantID,
		TokenType: "refresh",
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(m.cfg.RefreshSecret))
	if err != nil {
		return "", fmt.Errorf("jwt: sign refresh token: %w", err)
	}
	return signed, nil
}

// ParseAccessToken validates and parses an access token.
func (m *Manager) ParseAccessToken(tokenString string) (*Claims, error) {
	return m.parseToken(tokenString, m.cfg.AccessSecret, "access")
}

// ParseRefreshToken validates and parses a refresh token.
func (m *Manager) ParseRefreshToken(tokenString string) (*Claims, error) {
	return m.parseToken(tokenString, m.cfg.RefreshSecret, "refresh")
}

func (m *Manager) parseToken(tokenString, secret, expectedType string) (*Claims, error) {
	token, err := gojwt.ParseWithClaims(tokenString, &Claims{}, func(t *gojwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, gojwt.WithIssuer(m.cfg.Issuer), gojwt.WithExpirationRequired())

	if err != nil {
		return nil, fmt.Errorf("jwt: parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwt: invalid token claims")
	}

	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("jwt: wrong token type: expected %s got %s", expectedType, claims.TokenType)
	}

	return claims, nil
}

// ──────────────────────────────────────────────
// RS256 support (production upgrade path)
// ──────────────────────────────────────────────

// ParseRSAPrivateKey parses a PEM-encoded RSA private key.
func ParseRSAPrivateKey(pemKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("jwt: failed to parse PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse RSA private key: %w", err)
	}
	return key, nil
}

// ParseRSAPublicKey parses a PEM-encoded RSA public key.
func ParseRSAPublicKey(pemKey []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("jwt: failed to parse PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse RSA public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("jwt: not an RSA public key")
	}
	return rsaPub, nil
}
