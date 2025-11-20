package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/itsnoxius/simple-proxy/internal/database"
)

// Proxy handles HTTP reverse proxy requests
type Proxy struct {
	db    *database.DB
	debug bool
}

// New creates a new proxy instance
func New(db *database.DB, debug bool) *Proxy {
	p := &Proxy{db: db, debug: debug}
	if debug {
		log.Printf("[DEBUG] Creating new proxy instance")
	}
	return p
}

// debugLog logs a debug message only if debug mode is enabled
func (p *Proxy) debugLog(format string, v ...interface{}) {
	if p.debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// ServeHTTP handles incoming HTTP requests and proxies them to the configured backend
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.debugLog("=== Proxy Handler Called ===")
	p.debugLog("Method: %s", r.Method)
	p.debugLog("Host: %s", r.Host)
	p.debugLog("URL.Path: %s", r.URL.Path)
	p.debugLog("URL.RawPath: %s", r.URL.RawPath)
	p.debugLog("URL.RawQuery: %s", r.URL.RawQuery)
	p.debugLog("URL.Fragment: %s", r.URL.Fragment)
	p.debugLog("URL.String(): %s", r.URL.String())
	p.debugLog("RequestURI: %s", r.RequestURI)
	p.debugLog("RemoteAddr: %s", r.RemoteAddr)

	// Extract domain from Host header
	host := r.Host
	if host == "" {
		log.Printf("[ERROR] Missing Host header from %s", r.RemoteAddr)
		http.Error(w, "Missing Host header", http.StatusBadRequest)
		return
	}

	// Strip port from host if present (e.g., "example.com:80" -> "example.com")
	domainName := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		domainName = host[:idx]
		p.debugLog("Stripped port from host: %s -> %s", host, domainName)
	}

	p.debugLog("Looking up domain: %s", domainName)
	// Lookup domain in database
	domain, err := p.db.GetDomain(domainName)
	if err != nil {
		log.Printf("[ERROR] Failed to lookup domain in database: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if domain == nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Set default protocol if not specified
	if domain.Protocol == "" {
		domain.Protocol = "http"
	}

	p.debugLog("Found domain record: %s -> %s:%d (%s)", domainName, domain.IP, domain.Port, domain.Protocol)

	// Build target URL
	targetURL := fmt.Sprintf("%s://%s:%d", domain.Protocol, domain.IP, domain.Port)
	p.debugLog("Target URL: %s", targetURL)
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("[ERROR] Failed to parse target URL %s: %v", targetURL, err)
		http.Error(w, "Invalid target configuration", http.StatusInternalServerError)
		return
	}
	p.debugLog("Parsed target: %s", target.String())

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Modify the request to preserve the full original path and query parameters
	// We override the director to ensure the complete path is preserved exactly as received
	proxy.Director = func(req *http.Request) {
		// Set scheme and host from target
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host

		// Preserve the full original path exactly as received (including encoded paths)
		req.URL.Path = r.URL.Path
		req.URL.RawPath = r.URL.RawPath
		req.URL.RawQuery = r.URL.RawQuery
		req.URL.Fragment = r.URL.Fragment

		// Set host header
		req.Host = target.Host

		// Clear RequestURI as it's not valid in client requests
		req.RequestURI = ""

		p.debugLog("Director: Modified request URL to %s", req.URL.String())
		p.debugLog("Director: req.URL.Path=%s, req.URL.RawPath=%s", req.URL.Path, req.URL.RawPath)
		p.debugLog("Director: req.URL.RawQuery=%s, req.URL.Fragment=%s", req.URL.RawQuery, req.URL.Fragment)
		p.debugLog("Director: req.Host=%s, req.Method=%s", req.Host, req.Method)
		p.debugLog("Director: req.Proto=%s, req.ProtoMajor=%d", req.Proto, req.ProtoMajor)
		p.debugLog("Director: Request headers count: %d", len(req.Header))
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[ERROR] Proxy error for %s %s: %v", r.Method, r.URL.String(), err)
		http.Error(w, "Proxy error: "+err.Error(), http.StatusBadGateway)
	}

	// Serve the request
	p.debugLog("Proxying request %s %s to %s", r.Method, r.URL.String(), targetURL)
	proxy.ServeHTTP(w, r)
	p.debugLog("Completed proxying request %s %s", r.Method, r.URL.String())
}

// HealthCheck handles health check requests
func (p *Proxy) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"status":"ok"}`)
}
