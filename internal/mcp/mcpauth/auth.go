package mcpauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

// AuthMode indicates the type of authentication used.
type AuthMode string

const (
	AuthModeAPIKey AuthMode = "api_key"
)

// UserInfo contains authenticated user information.
type UserInfo struct {
	AuthMode  AuthMode
	Scopes    []string
	ExpiresAt time.Time
}

// tokenInfoKey is used to store TokenInfo in context.
type tokenInfoKey struct{}

// ContextWithTokenInfo adds TokenInfo to a context.
func ContextWithTokenInfo(ctx context.Context, info *auth.TokenInfo) context.Context {
	return context.WithValue(ctx, tokenInfoKey{}, info)
}

// TokenInfoFromContext retrieves TokenInfo from a context.
func TokenInfoFromContext(ctx context.Context) *auth.TokenInfo {
	if info, ok := ctx.Value(tokenInfoKey{}).(*auth.TokenInfo); ok {
		return info
	}
	return nil
}

// Authenticator handles MCP authentication via API key.
type Authenticator struct {
	apiKey     string
	apiKeyHash string
}

// NewAuthenticator creates a new MCP authenticator.
// Reads API key from DATASAVER_MCP_API_KEY environment variable.
func NewAuthenticator() *Authenticator {
	apiKey := os.Getenv("DATASAVER_MCP_API_KEY")
	var apiKeyHash string
	if apiKey != "" {
		apiKeyHash = HashToken(apiKey)
	}
	return &Authenticator{
		apiKey:     apiKey,
		apiKeyHash: apiKeyHash,
	}
}

// Enabled returns true if an API key is configured.
func (a *Authenticator) Enabled() bool {
	return a.apiKey != ""
}

// TokenVerifier returns a token verifier function for use with auth.RequireBearerToken.
func (a *Authenticator) TokenVerifier() func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
	return func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
		return a.verifyAPIKey(ctx, token)
	}
}

// verifyAPIKey validates an API key and returns token info.
func (a *Authenticator) verifyAPIKey(_ context.Context, apiKey string) (*auth.TokenInfo, error) {
	if a.apiKey == "" {
		return nil, auth.ErrInvalidToken
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(apiKey), []byte(a.apiKey)) != 1 {
		return nil, auth.ErrInvalidToken
	}

	userInfo := &UserInfo{
		AuthMode: AuthModeAPIKey,
		Scopes:   []string{"mcp:full"},
	}

	return &auth.TokenInfo{
		Scopes: userInfo.Scopes,
		Extra: map[string]any{
			"user_info": userInfo,
		},
	}, nil
}

// HashToken creates a SHA-256 hash of a token.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ValidateAuthHeader extracts and validates Bearer token from Authorization header.
func (a *Authenticator) ValidateAuthHeader(authHeader string) (*auth.TokenInfo, error) {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, auth.ErrInvalidToken
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return nil, auth.ErrInvalidToken
	}

	return a.verifyAPIKey(context.Background(), token)
}
