# HTTP Reverse Proxy

A Golang-based HTTP reverse proxy that routes requests based on domain names to configured IP/port backends. Configuration is stored in SQLite and can be managed via REST API endpoints.

## Features

-   HTTP reverse proxy based on Host header
-   SQLite database for domain configuration storage
-   REST API for dynamic configuration management
-   API key authentication for management endpoints
-   Domain-based access restriction for API endpoints
-   Support for HTTP and HTTPS backend protocols
-   Bulk domain creation endpoint
-   Returns 404 for unmapped domains

## Prerequisites

-   Go 1.21 or later
-   No CGO required (uses pure Go SQLite driver)

## Installation

1. Install dependencies:

```bash
go mod download
```

2. Set environment variables:

```bash
export PROXY_API_KEY=your-secret-api-key        # required
export PROXY_API_DOMAIN=api.example.com         # required - domain for API access
export DB_PATH=data/proxy.db                    # optional, defaults to data/proxy.db
export PORT=80                                   # optional, defaults to 80
export DEBUG=false                               # optional, defaults to false
```

## Running

```bash
go run cmd/proxy/main.go
```

Or build and run:

```bash
go build -o proxy cmd/proxy/main.go
./proxy
```

## Docker

### Building the Docker Image

```bash
docker build -t proxy .
```

### Running with Docker

```bash
docker run -d \
  --name proxy \
  -p 80:80 \
  -e PROXY_API_KEY=your-secret-api-key \
  -e PROXY_API_DOMAIN=api.example.com \
  -e DB_PATH=/app/data/proxy.db \
  -e PORT=80 \
  -e DEBUG=false \
  -v $(pwd)/data:/app/data \
  proxy
```

### Using Docker Compose

1. Create a `.env` file (or set environment variables):

```bash
PROXY_API_KEY=your-secret-api-key-change-me
PROXY_API_DOMAIN=api.example.com
DB_PATH=/app/data/proxy.db
PORT=80
DEBUG=false
```

2. Build and run:

```bash
docker-compose up -d
```

3. View logs:

```bash
docker-compose logs -f
```

4. Stop the container:

```bash
docker-compose down
```

The database will be persisted in the `./data` directory.

## API Endpoints

All API endpoints require:

-   Authentication via `Authorization: Bearer <PROXY_API_KEY>` header
-   Access from the domain specified in `PROXY_API_DOMAIN` environment variable (domain restriction middleware)

### List all domain mappings

```
GET /api/config
```

Response:

```json
[
    {
        "domain": "example.com",
        "ip": "192.168.1.100",
        "port": 8080,
        "protocol": "http",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
    }
]
```

### Get specific domain mapping

```
GET /api/config/:domain
```

### Create domain mapping

```
POST /api/config
Content-Type: application/json

{
  "domain": "example.com",
  "ip": "192.168.1.100",
  "port": 8080,
  "protocol": "http"
}
```

Note: `protocol` is optional and defaults to `"http"` if not provided.

### Update domain mapping

```
PUT /api/config/:domain
Content-Type: application/json

{
  "ip": "192.168.1.200",
  "port": 8080,
  "protocol": "https"
}
```

Note: `protocol` is optional and will preserve the existing value if not provided.

### Delete domain mapping

```
DELETE /api/config/:domain
```

### Bulk create domain mappings

```
POST /api/config/bulk
Content-Type: application/json

[
  {
    "domain": "example.com",
    "ip": "192.168.1.100",
    "port": 8080,
    "protocol": "http"
  },
  {
    "domain": "another.com",
    "ip": "192.168.1.101",
    "port": 8080
  }
]
```

Or using the object format:

```json
{
    "domains": [
        {
            "domain": "example.com",
            "ip": "192.168.1.100",
            "port": 8080,
            "protocol": "http"
        }
    ]
}
```

## Health Check

```
GET /health
```

Returns:

```json
{ "status": "ok" }
```

## Whoami Endpoint

```
GET /whoami
```

Returns diagnostic information about the request and server, including:

-   Hostname
-   Server IP addresses
-   Remote address
-   Request details
-   Optional query parameters:
    -   `?wait=<duration>` - Wait for specified duration before responding (e.g., `?wait=5s`)
    -   `?env=true` - Include environment variables in response

## Example Usage

1. Start the proxy server:

```bash
export PROXY_API_KEY=my-secret-key
export PROXY_API_DOMAIN=api.example.com
go run cmd/proxy/main.go
```

2. Create a domain mapping (note: API must be accessed via the PROXY_API_DOMAIN):

```bash
curl -X POST http://api.example.com/api/config \
  -H "Authorization: Bearer my-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com","ip":"192.168.1.100","port":8080,"protocol":"http"}'
```

3. Test the proxy:

```bash
curl -H "Host: example.com" http://localhost/
```

This will proxy the request to `http://192.168.1.100:8080/`

## Configuration

Environment variables:

-   `PROXY_API_KEY` (required): API key for authentication
-   `PROXY_API_DOMAIN` (required): Domain name that API endpoints must be accessed from (e.g., `api.example.com`)
-   `DB_PATH` (optional): Path to SQLite database file (default: `data/proxy.db`)
-   `PORT` (optional): Server port (default: `80`)
-   `DEBUG` (optional): Enable debug logging (default: `false`)

## Database Schema

The SQLite database contains a `domains` table with the following schema:

-   `domain`: TEXT PRIMARY KEY NOT NULL
-   `ip`: TEXT NOT NULL
-   `port`: INTEGER NOT NULL DEFAULT 80
-   `protocol`: TEXT NOT NULL DEFAULT 'http'
-   `created_at`: DATETIME DEFAULT CURRENT_TIMESTAMP
-   `updated_at`: DATETIME DEFAULT CURRENT_TIMESTAMP

## License

MIT
