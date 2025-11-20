package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/itsnoxius/simple-proxy/internal/database"
	"github.com/itsnoxius/simple-proxy/pkg/models"
)

// Handlers contains HTTP handlers for the API
type Handlers struct {
	db        *database.DB
	apiKey    string
	authToken string
}

// NewHandlers creates a new handlers instance
func NewHandlers(db *database.DB, apiKey string) *Handlers {
	return &Handlers{
		db:        db,
		apiKey:    apiKey,
		authToken: "Bearer " + apiKey,
	}
}

// DomainMiddleware validates that requests come from the allowed domain
func DomainMiddleware(allowedDomain string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Strip port from host if present (e.g., "example.com:80" -> "example.com")
			domainName := host
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				domainName = host[:idx]
			}

			if domainName != allowedDomain {
				http.Error(w, "Forbidden: API access restricted to specific domain", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware validates API key authentication
func (h *Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != h.authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListDomains handles GET /api/config
func (h *Handlers) ListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.db.GetAllDomains()
	if err != nil {
		http.Error(w, "Failed to retrieve domains: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// GetDomain handles GET /api/config/:domain
func (h *Handlers) GetDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain, err := url.PathUnescape(vars["domain"])
	if err != nil {
		http.Error(w, "Invalid domain parameter", http.StatusBadRequest)
		return
	}

	domainModel, err := h.db.GetDomain(domain)
	if err != nil {
		http.Error(w, "Failed to retrieve domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if domainModel == nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domainModel)
}

// CreateDomain handles POST /api/config
func (h *Handlers) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Domain == "" || req.IP == "" || req.Port == 0 {
		http.Error(w, "Missing required fields: domain, ip, port", http.StatusBadRequest)
		return
	}

	// Set default protocol if not provided
	if req.Protocol == "" {
		req.Protocol = "http"
	}

	domain, err := h.db.CreateDomain(req)
	if err != nil {
		http.Error(w, "Failed to create domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(domain)
}

// UpdateDomain handles PUT /api/config/:domain
func (h *Handlers) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain, err := url.PathUnescape(vars["domain"])
	if err != nil {
		http.Error(w, "Invalid domain parameter", http.StatusBadRequest)
		return
	}

	var req models.UpdateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.IP == "" || req.Port == 0 {
		http.Error(w, "Missing required fields: ip, port", http.StatusBadRequest)
		return
	}

	// Set default protocol if not provided (will be handled by database method)
	if req.Protocol == "" {
		req.Protocol = "http"
	}

	domainModel, err := h.db.UpdateDomain(domain, req)
	if err != nil {
		http.Error(w, "Failed to update domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if domainModel == nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domainModel)
}

// DeleteDomain handles DELETE /api/config/:domain
func (h *Handlers) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain, err := url.PathUnescape(vars["domain"])
	if err != nil {
		http.Error(w, "Invalid domain parameter", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteDomain(domain); err != nil {
		if err.Error() == "domain not found" {
			http.Error(w, "Domain not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BulkCreateDomains handles POST /api/config/bulk
func (h *Handlers) BulkCreateDomains(w http.ResponseWriter, r *http.Request) {
	// Read the body once
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var domains []models.CreateDomainRequest
	// Try to decode as array first (most common case)
	if err := json.Unmarshal(bodyBytes, &domains); err != nil {
		// Try to decode as BulkCreateDomainsRequest format for backward compatibility
		var req models.BulkCreateDomainsRequest
		if err2 := json.Unmarshal(bodyBytes, &req); err2 != nil {
			http.Error(w, "Invalid request body: expected array of domains or object with 'domains' field", http.StatusBadRequest)
			return
		}
		domains = req.Domains
	}

	// Validate that domains array is not empty
	if len(domains) == 0 {
		http.Error(w, "No domains provided", http.StatusBadRequest)
		return
	}

	// Validate all domains have required fields
	for i, domain := range domains {
		if domain.Domain == "" || domain.IP == "" || domain.Port == 0 {
			http.Error(w, fmt.Sprintf("Missing required fields in domain at index %d: domain, ip, port", i), http.StatusBadRequest)
			return
		}
		// Set default protocol if not provided
		if domain.Protocol == "" {
			domains[i].Protocol = "http"
		}
	}

	createdDomains, err := h.db.BulkCreateDomains(domains)
	if err != nil {
		http.Error(w, "Failed to create domains: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdDomains)
}
