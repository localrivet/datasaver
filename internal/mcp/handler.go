package mcp

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/almatuck/datasaver/internal/config"
	"github.com/almatuck/datasaver/internal/notify"
	"github.com/almatuck/datasaver/internal/storage"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler handles MCP HTTP requests with simple API key authentication.
type Handler struct {
	cfg         *config.Config
	storage     storage.Backend
	notifier    *notify.Notifier
	logger      *slog.Logger
	apiKey      string
	httpHandler http.Handler
}

// NewHandler creates a new MCP handler with API key authentication.
// API key is read from DATASAVER_MCP_API_KEY environment variable.
func NewHandler(cfg *config.Config, store storage.Backend, notifier *notify.Notifier, logger *slog.Logger) *Handler {
	apiKey := os.Getenv("DATASAVER_MCP_API_KEY")
	if apiKey == "" {
		logger.Warn("DATASAVER_MCP_API_KEY not set - MCP endpoint will reject all requests")
	}

	h := &Handler{
		cfg:      cfg,
		storage:  store,
		notifier: notifier,
		logger:   logger,
		apiKey:   apiKey,
	}

	// Create the streamable HTTP handler
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

// authMiddleware validates API key from Authorization header.
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log request for debugging
		sessionID := r.Header.Get("Mcp-Session-Id")
		h.logger.Debug("MCP request", "method", r.Method, "path", r.URL.Path, "session", sessionID)

		// Check if API key is configured
		if h.apiKey == "" {
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

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.apiKey)) != 1 {
			h.writeUnauthorized(w, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeUnauthorized sends a 401 response.
func (h *Handler) writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="datasaver"`)
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
	return h.apiKey != ""
}
