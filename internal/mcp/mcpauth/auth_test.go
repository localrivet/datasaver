package mcpauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewAuthenticator_NoAPIKey(t *testing.T) {
	os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()
	if auth.Enabled() {
		t.Error("Expected Enabled() to be false when no API key is set")
	}
}

func TestNewAuthenticator_WithAPIKey(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "test-api-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()
	if !auth.Enabled() {
		t.Error("Expected Enabled() to be true when API key is set")
	}
}

func TestAuthenticator_TokenVerifier_ValidKey(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "test-api-key-123")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()
	verifier := auth.TokenVerifier()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), "test-api-key-123", req)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if tokenInfo == nil {
		t.Fatal("Expected tokenInfo, got nil")
	}
	if len(tokenInfo.Scopes) != 1 || tokenInfo.Scopes[0] != "mcp:full" {
		t.Errorf("Expected scopes [mcp:full], got %v", tokenInfo.Scopes)
	}

	userInfo, ok := tokenInfo.Extra["user_info"].(*UserInfo)
	if !ok {
		t.Fatal("Expected user_info in Extra")
	}
	if userInfo.AuthMode != AuthModeAPIKey {
		t.Errorf("Expected AuthMode to be api_key, got %s", userInfo.AuthMode)
	}
}

func TestAuthenticator_TokenVerifier_InvalidKey(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "correct-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()
	verifier := auth.TokenVerifier()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	_, err := verifier(context.Background(), "wrong-key", req)

	if err == nil {
		t.Error("Expected error for invalid key")
	}
}

func TestAuthenticator_TokenVerifier_NoKeyConfigured(t *testing.T) {
	os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()
	verifier := auth.TokenVerifier()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	_, err := verifier(context.Background(), "any-key", req)

	if err == nil {
		t.Error("Expected error when no API key is configured")
	}
}

func TestAuthenticator_ValidateAuthHeader_ValidBearer(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "my-secret-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()

	tokenInfo, err := auth.ValidateAuthHeader("Bearer my-secret-key")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if tokenInfo == nil {
		t.Fatal("Expected tokenInfo, got nil")
	}
}

func TestAuthenticator_ValidateAuthHeader_MissingBearer(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "my-secret-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()

	_, err := auth.ValidateAuthHeader("my-secret-key")
	if err == nil {
		t.Error("Expected error for missing Bearer prefix")
	}
}

func TestAuthenticator_ValidateAuthHeader_EmptyHeader(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "my-secret-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()

	_, err := auth.ValidateAuthHeader("")
	if err == nil {
		t.Error("Expected error for empty header")
	}
}

func TestAuthenticator_ValidateAuthHeader_EmptyToken(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "my-secret-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()

	_, err := auth.ValidateAuthHeader("Bearer ")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

func TestHashToken(t *testing.T) {
	hash1 := HashToken("test-token")
	hash2 := HashToken("test-token")
	hash3 := HashToken("different-token")

	if hash1 != hash2 {
		t.Error("Same token should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("Different tokens should produce different hashes")
	}
	if len(hash1) != 64 {
		t.Errorf("Expected 64 character hex hash, got %d", len(hash1))
	}
}

func TestContextWithTokenInfo(t *testing.T) {
	os.Setenv("DATASAVER_MCP_API_KEY", "test-key")
	defer os.Unsetenv("DATASAVER_MCP_API_KEY")

	auth := NewAuthenticator()
	verifier := auth.TokenVerifier()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, _ := verifier(context.Background(), "test-key", req)

	ctx := ContextWithTokenInfo(context.Background(), tokenInfo)
	retrieved := TokenInfoFromContext(ctx)

	if retrieved == nil {
		t.Fatal("Expected to retrieve tokenInfo from context")
	}
	if len(retrieved.Scopes) != 1 || retrieved.Scopes[0] != "mcp:full" {
		t.Errorf("Expected scopes to match, got %v", retrieved.Scopes)
	}
}

func TestTokenInfoFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	tokenInfo := TokenInfoFromContext(ctx)

	if tokenInfo != nil {
		t.Error("Expected nil tokenInfo from empty context")
	}
}
