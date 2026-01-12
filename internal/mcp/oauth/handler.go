package oauth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler provides MCP OAuth discovery endpoints.
// Datasaver uses API key auth, but MCP clients need OAuth discovery to find the auth method.
type Handler struct {
	baseURL string
}

// NewHandler creates a new OAuth discovery handler.
func NewHandler(baseURL string) *Handler {
	return &Handler{
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// RegisterRoutes registers OAuth discovery routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.HandleProtectedResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.HandleAuthServerMetadata)
}

// setCORSHeaders sets CORS headers for OAuth endpoints.
func (h *Handler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// HandleProtectedResourceMetadata returns the protected resource metadata.
// Per RFC 9728, this tells clients where to find the auth server.
func (h *Handler) HandleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	metadata := ProtectedResourceMetadata{
		Resource:               h.baseURL + "/mcp",
		AuthorizationServers:   []string{h.baseURL},
		ScopesSupported:        []string{"mcp:full"},
		BearerMethodsSupported: []string{"header"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	json.NewEncoder(w).Encode(metadata)
}

// HandleAuthServerMetadata returns the authorization server metadata.
// Datasaver uses pre-shared API keys, so we advertise minimal capabilities.
func (h *Handler) HandleAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Datasaver uses API keys, not full OAuth flow.
	// We still provide metadata for MCP client compatibility.
	metadata := AuthorizationServerMetadata{
		Issuer:                            h.baseURL,
		TokenEndpoint:                     h.baseURL + "/oauth/token",
		ScopesSupported:                   []string{"mcp:full"},
		ResponseTypesSupported:            []string{}, // No authorization code flow
		GrantTypesSupported:               []string{}, // API key only
		TokenEndpointAuthMethodsSupported: []string{"bearer"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	json.NewEncoder(w).Encode(metadata)
}
