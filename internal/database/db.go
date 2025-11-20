package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/itsnoxius/simple-proxy/pkg/models"
	_ "modernc.org/sqlite"
)

// DB wraps the database connection and operations
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}

	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the necessary tables
func (db *DB) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS domains (
		domain TEXT PRIMARY KEY NOT NULL,
		ip TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 80,
		protocol TEXT NOT NULL DEFAULT 'http',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.conn.Exec(query)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// GetDomain retrieves a domain mapping by domain name
func (db *DB) GetDomain(domain string) (*models.Domain, error) {
	query := `SELECT domain, ip, port, protocol, created_at, updated_at FROM domains WHERE domain = ?`
	row := db.conn.QueryRow(query, domain)

	var d models.Domain
	var createdAt, updatedAt string

	err := row.Scan(&d.Domain, &d.IP, &d.Port, &d.Protocol, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	// Parse timestamps - SQLite can return various formats
	parseTime := func(ts string) time.Time {
		formats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05",
			time.RFC3339,
		}
		for _, format := range formats {
			if t, err := time.Parse(format, ts); err == nil {
				return t
			}
		}
		return time.Time{}
	}
	d.CreatedAt = parseTime(createdAt)
	d.UpdatedAt = parseTime(updatedAt)

	return &d, nil
}

// GetAllDomains retrieves all domain mappings
func (db *DB) GetAllDomains() ([]models.Domain, error) {
	query := `SELECT domain, ip, port, protocol, created_at, updated_at FROM domains ORDER BY domain`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer rows.Close()

	var domains []models.Domain
	for rows.Next() {
		var d models.Domain
		var createdAt, updatedAt string

		err := rows.Scan(&d.Domain, &d.IP, &d.Port, &d.Protocol, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}

		d.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		d.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		domains = append(domains, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating domains: %w", err)
	}

	return domains, nil
}

// CreateDomain creates a new domain mapping
func (db *DB) CreateDomain(req models.CreateDomainRequest) (*models.Domain, error) {
	protocol := req.Protocol
	if protocol == "" {
		protocol = "http" // Default to http if not specified
	}
	query := `INSERT INTO domains (domain, ip, port, protocol) VALUES (?, ?, ?, ?)`
	_, err := db.conn.Exec(query, req.Domain, req.IP, req.Port, protocol)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain: %w", err)
	}

	// Fetch the created domain
	return db.GetDomain(req.Domain)
}

// UpdateDomain updates an existing domain mapping
func (db *DB) UpdateDomain(domain string, req models.UpdateDomainRequest) (*models.Domain, error) {
	protocol := req.Protocol
	if protocol == "" {
		// If protocol not provided, keep existing value by selecting it first
		existing, err := db.GetDomain(domain)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing domain: %w", err)
		}
		if existing == nil {
			return nil, nil
		}
		protocol = existing.Protocol
	}
	query := `UPDATE domains SET ip = ?, port = ?, protocol = ?, updated_at = CURRENT_TIMESTAMP WHERE domain = ?`
	result, err := db.conn.Exec(query, req.IP, req.Port, protocol, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to update domain: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, nil
	}

	// Fetch the updated domain
	return db.GetDomain(domain)
}

// DeleteDomain deletes a domain mapping
func (db *DB) DeleteDomain(domain string) error {
	query := `DELETE FROM domains WHERE domain = ?`
	result, err := db.conn.Exec(query, domain)
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("domain not found")
	}

	return nil
}

// BulkCreateDomains creates multiple domain mappings in a single transaction
func (db *DB) BulkCreateDomains(domains []models.CreateDomainRequest) ([]models.Domain, error) {
	if len(domains) == 0 {
		return []models.Domain{}, nil
	}

	// Start a transaction
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO domains (domain, ip, port, protocol) VALUES (?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	var createdDomains []models.Domain
	var domainNames []string

	// Insert all domains
	for _, req := range domains {
		protocol := req.Protocol
		if protocol == "" {
			protocol = "http" // Default to http if not specified
		}
		_, err := stmt.Exec(req.Domain, req.IP, req.Port, protocol)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain %s: %w", req.Domain, err)
		}
		domainNames = append(domainNames, req.Domain)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch all created domains
	for _, domainName := range domainNames {
		domain, err := db.GetDomain(domainName)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch created domain %s: %w", domainName, err)
		}
		if domain != nil {
			createdDomains = append(createdDomains, *domain)
		}
	}

	return createdDomains, nil
}
