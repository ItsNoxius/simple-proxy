package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/itsnoxius/simple-proxy/internal/api"
	"github.com/itsnoxius/simple-proxy/internal/config"
	"github.com/itsnoxius/simple-proxy/internal/database"
	"github.com/itsnoxius/simple-proxy/internal/proxy"
)

var (
	name string
	cfg  *config.Config
	db   *database.DB
)

func debugLog(format string, v ...interface{}) {
	if cfg != nil && cfg.Debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func init() {
	cfg = config.Load()
	debugLog("Initializing application...")
	debugLog("Configuration loaded: Port=%d, DBPath=%s, Debug=%v", cfg.Port, cfg.DBPath, cfg.Debug)

	// Validate API key is set
	if cfg.ProxyAPIKey == "" {
		log.Fatal("PROXY_API_KEY environment variable is required")
	}
	debugLog("API key validated")

	// Validate API domain is set
	if cfg.APIDomain == "" {
		log.Fatal("PROXY_API_DOMAIN environment variable is required")
	}
	debugLog("API domain validated: %s", cfg.APIDomain)

	var err error
	db, err = database.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	debugLog("Database initialized at: %s", cfg.DBPath)
}

type DomainedResponse struct {
	Domain     string            `json:"domain"`
	IP         string            `json:"ip"`
	Port       int               `json:"port"`
	Protocol   string            `json:"protocol"`
	Headers    map[string]string `json:"headers"`
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Body       string            `json:"body"`
	StatusCode int               `json:"status_code"`
	StatusText string            `json:"status_text"`
}

func main() {
	debugLog("Starting main function...")
	defer func() {
		if db != nil {
			db.Close()
		}
	}()
	router := mux.NewRouter()

	proxyHandler := proxy.New(db, cfg.Debug)
	debugLog("Proxy handler created")

	// Initialize API handlers
	apiHandlers := api.NewHandlers(db, cfg.ProxyAPIKey)
	debugLog("API handlers created")

	// Create API subrouter with domain middleware
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(api.DomainMiddleware(cfg.APIDomain))
	apiRouter.Use(apiHandlers.AuthMiddleware)

	// Register API routes
	// Note: More specific routes should be registered first
	apiRouter.HandleFunc("/config/bulk", apiHandlers.BulkCreateDomains).Methods("POST")
	apiRouter.HandleFunc("/config", apiHandlers.ListDomains).Methods("GET")
	apiRouter.HandleFunc("/config", apiHandlers.CreateDomain).Methods("POST")
	apiRouter.HandleFunc("/config/{domain}", apiHandlers.GetDomain).Methods("GET")
	apiRouter.HandleFunc("/config/{domain}", apiHandlers.UpdateDomain).Methods("PUT")
	apiRouter.HandleFunc("/config/{domain}", apiHandlers.DeleteDomain).Methods("DELETE")
	debugLog("Registered API routes with domain protection: %s", cfg.APIDomain)

	// Register specific routes first (these take precedence)
	router.HandleFunc("/whoami", whoamiHandler)
	debugLog("Registered route: /whoami")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		debugLog("Health check requested from %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok"}`)
	})
	debugLog("Registered route: /health")

	// Register the proxy handler as a catch-all for all other paths
	// PathPrefix("/") matches all paths, ensuring all requests go through the proxy handler
	router.PathPrefix("/").Handler(proxyHandler)
	debugLog("Registered catch-all proxy handler")

	// Start the HTTP server on port 80
	port := fmt.Sprintf(":%d", cfg.Port)

	log.Printf("[INFO] Starting server on port %s", port)
	err := http.ListenAndServe(port, router) // The 'nil' argument uses the default ServeMux
	if err != nil {
		log.Fatalf("[FATAL] Server failed to start: %v", err)
	}
}

func getIPs() []string {
	var ips []string

	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips
}

func whoamiHandler(w http.ResponseWriter, r *http.Request) {
	debugLog("whoami handler called from %s", r.RemoteAddr)
	queryParams := r.URL.Query()

	wait := queryParams.Get("wait")
	if wait != "" {
		debugLog("whoami: wait parameter=%s", wait)
		duration, err := time.ParseDuration(wait)
		if err == nil {
			debugLog("whoami: sleeping for %v", duration)
			time.Sleep(duration)
		} else {
			log.Printf("[WARN] whoami: invalid wait duration %s: %v", wait, err)
		}
	}

	_, _ = fmt.Fprintln(w, "Custom Header")

	if name != "" {
		_, _ = fmt.Fprintln(w, "Name:", name)
	}

	hostname, _ := os.Hostname()
	_, _ = fmt.Fprintln(w, "Hostname:", hostname)

	for _, ip := range getIPs() {
		_, _ = fmt.Fprintln(w, "IP:", ip)
	}

	_, _ = fmt.Fprintln(w, "RemoteAddr:", r.RemoteAddr)

	if r.TLS != nil {
		for i, cert := range r.TLS.PeerCertificates {
			_, _ = fmt.Fprintf(w, "Certificate[%d] Subject: %v\n", i, cert.Subject)
		}
	}

	if err := r.Write(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ok, _ := strconv.ParseBool(queryParams.Get("env")); ok {
		for _, env := range os.Environ() {
			_, _ = fmt.Fprintln(w, env)
		}
	}
}
