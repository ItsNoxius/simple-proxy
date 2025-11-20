package models

import "time"

// Domain represents a domain mapping configuration
type Domain struct {
	Domain    string    `json:"domain"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateDomainRequest represents a request to create a new domain mapping
type CreateDomainRequest struct {
	Domain   string `json:"domain"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// UpdateDomainRequest represents a request to update a domain mapping
type UpdateDomainRequest struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// BulkCreateDomainsRequest represents a request to create multiple domain mappings
type BulkCreateDomainsRequest struct {
	Domains []CreateDomainRequest `json:"domains"`
}
