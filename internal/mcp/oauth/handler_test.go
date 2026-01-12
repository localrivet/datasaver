package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler("https://example.com")
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestNewHandler_TrailingSlash(t *testing.T) {
	h := NewHandler("https://example.com/")
	if h.baseURL != "https://example.com" {
		t.Errorf("Expected trailing slash to be removed, got %s", h.baseURL)
	}
}

func TestHandler_ProtectedResourceMetadata(t *testing.T) {
	h := NewHandler("https://backup.example.com")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	w := httptest.NewRecorder()

	h.HandleProtectedResourceMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var metadata ProtectedResourceMetadata
	if err := json.NewDecoder(w.Body).Decode(&metadata); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if metadata.Resource != "https://backup.example.com/mcp" {
		t.Errorf("Expected resource to be https://backup.example.com/mcp, got %s", metadata.Resource)
	}

	if len(metadata.AuthorizationServers) != 1 || metadata.AuthorizationServers[0] != "https://backup.example.com" {
		t.Errorf("Expected authorization server to be https://backup.example.com, got %v", metadata.AuthorizationServers)
	}

	if len(metadata.ScopesSupported) != 1 || metadata.ScopesSupported[0] != "mcp:full" {
		t.Errorf("Expected scopes to be [mcp:full], got %v", metadata.ScopesSupported)
	}

	if len(metadata.BearerMethodsSupported) != 1 || metadata.BearerMethodsSupported[0] != "header" {
		t.Errorf("Expected bearer methods to be [header], got %v", metadata.BearerMethodsSupported)
	}
}

func TestHandler_AuthServerMetadata(t *testing.T) {
	h := NewHandler("https://backup.example.com")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	h.HandleAuthServerMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var metadata AuthorizationServerMetadata
	if err := json.NewDecoder(w.Body).Decode(&metadata); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if metadata.Issuer != "https://backup.example.com" {
		t.Errorf("Expected issuer to be https://backup.example.com, got %s", metadata.Issuer)
	}

	if metadata.TokenEndpoint != "https://backup.example.com/oauth/token" {
		t.Errorf("Expected token endpoint to be https://backup.example.com/oauth/token, got %s", metadata.TokenEndpoint)
	}

	if len(metadata.ScopesSupported) != 1 || metadata.ScopesSupported[0] != "mcp:full" {
		t.Errorf("Expected scopes to be [mcp:full], got %v", metadata.ScopesSupported)
	}
}

func TestHandler_ProtectedResourceMetadata_CORS(t *testing.T) {
	h := NewHandler("https://example.com")

	req := httptest.NewRequest(http.MethodOptions, "/.well-known/oauth-protected-resource", nil)
	w := httptest.NewRecorder()

	h.HandleProtectedResourceMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin: *")
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected Access-Control-Allow-Methods header")
	}
}

func TestHandler_AuthServerMetadata_CORS(t *testing.T) {
	h := NewHandler("https://example.com")

	req := httptest.NewRequest(http.MethodOptions, "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	h.HandleAuthServerMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin: *")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	h := NewHandler("https://example.com")
	mux := http.NewServeMux()

	h.RegisterRoutes(mux)

	// Test protected resource endpoint
	req1 := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected protected resource endpoint to return 200, got %d", w1.Code)
	}

	// Test auth server endpoint
	req2 := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected auth server endpoint to return 200, got %d", w2.Code)
	}
}

func TestHandler_CacheControl(t *testing.T) {
	h := NewHandler("https://example.com")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	w := httptest.NewRecorder()

	h.HandleProtectedResourceMetadata(w, req)

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-store, no-cache, must-revalidate" {
		t.Errorf("Expected Cache-Control to prevent caching, got %s", cacheControl)
	}
}
