package mcp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/localrivet/datasaver/internal/config"
	"github.com/localrivet/datasaver/internal/mcp/mcpauth"
	"github.com/localrivet/datasaver/internal/notify"
	"github.com/localrivet/datasaver/internal/storage"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler handles MCP HTTP requests with Bearer token authentication.
// Supports API keys via DATASAVER_MCP_API_KEY environment variable.
type Handler struct {
	cfg             *config.Config
	storage         storage.Backend
	notifier        *notify.Notifier
	logger          *slog.Logger
	authenticator   *mcpauth.Authenticator
	httpHandler     http.Handler
	resourceMetaURL string
}

// NewHandler creates a new MCP handler with authentication.
// baseURL is used to construct the resource metadata URL for OAuth discovery.
func NewHandler(cfg *config.Config, store storage.Backend, notifier *notify.Notifier, logger *slog.Logger, baseURL string) *Handler {
	baseURL = strings.TrimSuffix(baseURL, "/")

	h := &Handler{
		cfg:             cfg,
		storage:         store,
		notifier:        notifier,
		logger:          logger,
		authenticator:   mcpauth.NewAuthenticator(),
		resourceMetaURL: baseURL + "/.well-known/oauth-protected-resource",
	}

	if !h.authenticator.Enabled() {
		logger.Warn("DATASAVER_MCP_API_KEY not set - MCP endpoint will reject all requests")
	}

	// Create the streamable HTTP handler in stateless mode.
	streamHandler := mcp.NewStreamableHTTPHandler(
		h.getServerForRequest,
		&mcp.StreamableHTTPOptions{
			Stateless: true,
		},
	)

	// Wrap with auth middleware
	h.httpHandler = h.authMiddleware(streamHandler)

	return h
}

// authMiddleware validates Bearer tokens and returns proper OAuth challenge on 401.
// Uses WWW-Authenticate header with resource_metadata for RFC 9728 compliance.
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log session info for debugging
		sessionID := r.Header.Get("Mcp-Session-Id")
		h.logger.Debug("MCP request",
			"method", r.Method,
			"path", r.URL.Path,
			"session", sessionID,
			"accept", r.Header.Get("Accept"),
		)

		// Check if API key is configured
		if !h.authenticator.Enabled() {
			http.Error(w, "MCP endpoint not configured", http.StatusServiceUnavailable)
			return
		}

		// Extract Bearer token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			h.writeUnauthorized(w, "missing bearer token")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			h.writeUnauthorized(w, "empty bearer token")
			return
		}

		// Verify token
		tokenInfo, err := h.authenticator.TokenVerifier()(r.Context(), token, r)
		if err != nil {
			h.writeUnauthorized(w, "invalid token")
			return
		}

		// Add token info to request context and continue
		ctx := mcpauth.ContextWithTokenInfo(r.Context(), tokenInfo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeUnauthorized sends a 401 response with WWW-Authenticate header for OAuth discovery.
// Uses RFC 9728 compliant format with quoted resource_metadata value.
func (h *Handler) writeUnauthorized(w http.ResponseWriter, _ string) {
	wwwAuth := fmt.Sprintf(`Bearer resource_metadata="%s", scope="mcp:full"`, h.resourceMetaURL)
	w.Header().Set("WWW-Authenticate", wwwAuth)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// getServerForRequest creates a new MCP server for each request.
func (h *Handler) getServerForRequest(r *http.Request) *mcp.Server {
	return NewServer(r.Context(), h.cfg, h.storage, h.notifier, h.logger)
}

// ServeHTTP handles all MCP HTTP requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.httpHandler.ServeHTTP(w, r)
}

// Enabled returns true if MCP is configured (API key is set).
func (h *Handler) Enabled() bool {
	return h.authenticator.Enabled()
}

// Authenticator returns the authenticator for external use.
func (h *Handler) Authenticator() *mcpauth.Authenticator {
	return h.authenticator
}
