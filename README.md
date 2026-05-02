# openvpn-admin

A Go-based admin web UI and management service for OpenVPN. Wraps an existing
[`openvpn-install`](https://github.com/angristan/openvpn-install)-style server
(EasyRSA PKI + `openvpn-server@server.service`) with a single-binary HTTP server,
a SPA dashboard, a REST API, and a CLI.

## Features

- **Web dashboard**: live sessions, traffic stats, certificate inventory, journalctl logs
- **User management**: create / revoke clients, download `.ovpn` configs
- **PKI integration**: reads `easy-rsa/pki/index.txt` and `openvpn-status.log` directly
- **REST API** with JWT-based admin authentication
- **CLI** for scripted user/config/backup operations
- **SQLite** for audit log and metadata
- **Single static binary** + dashboard

## Deployment

Two supported deployment methods. Pick one.

### Option A — Local (binary + systemd)

For installations on the same host as the OpenVPN server, with direct access to
`/etc/openvpn` and the `openvpn-server@server.service` unit.

```bash
# Build
git clone https://github.com/shutcode/openvpn-admin.git
cd openvpn-admin
go build -o openvpn-mgmt ./cmd/server

# Install binary + dashboard
sudo mkdir -p /opt/openvpn-mgmt/data
sudo cp openvpn-mgmt /opt/openvpn-mgmt/
sudo cp -r dashboard /opt/openvpn-mgmt/

# Generate JWT + admin password
openssl rand -hex 32 | sudo tee /opt/openvpn-mgmt/.jwt_secret >/dev/null
sudo chmod 600 /opt/openvpn-mgmt/.jwt_secret

# Install systemd unit
sudo cp scripts/openvpn-mgmt.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now openvpn-mgmt

# Verify
curl http://localhost:8080/health
```

The systemd unit (`scripts/openvpn-mgmt.service`) reads `JWT_SECRET_FILE`,
`EASYRSA_PATH`, `OPENVPN_PATH`, and `CLIENTS_DIR` from `Environment=` lines —
edit it to override defaults. Set `ADMIN_USER` / `ADMIN_PASSWORD` (or
`ADMIN_PASSWORD_FILE`) for dashboard login.

> **SELinux note**: on RHEL/CentOS/Rocky, after copying the binary into
> `/opt/openvpn-mgmt/`, run `sudo chcon -t bin_t /opt/openvpn-mgmt/openvpn-mgmt`
> so systemd can execute it.

### Option B — Docker Compose

For installations where you want isolation, or where OpenVPN itself runs in a
container. Mount the host's OpenVPN directories into the container.

```bash
git clone https://github.com/shutcode/openvpn-admin.git
cd openvpn-admin

# Required secrets
export JWT_SECRET=$(openssl rand -hex 32)
export ADMIN_PASSWORD=$(openssl rand -hex 16)

# Bring up the stack
docker compose up -d

# Tail logs
docker compose logs -f openvpn-mgmt

# Verify
curl http://localhost:8282/health
```

The default `docker-compose.yml` creates named volumes for `/etc/openvpn`,
`easy-rsa`, and `clients`. To bind to an existing host OpenVPN install, edit
the `volumes:` block:

```yaml
volumes:
  - ./data:/data
  - /etc/openvpn:/etc/openvpn
  - /etc/openvpn/easy-rsa:/etc/openvpn/easy-rsa
  - /etc/openvpn/clients:/etc/openvpn/clients
```

Optional reverse proxy with HTTPS:

```bash
mkdir -p ssl
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout ssl/key.pem -out ssl/cert.pem \
  -subj "/CN=$(hostname)"
docker compose --profile with-nginx up -d
```

See [DOCKER.md](DOCKER.md) for the full Compose reference.

> **Behind the GFW?** `Dockerfile.local` + `docker-compose.local.yml` use the
> `docker.1ms.run` mirror and `goproxy.cn` for environments where Docker Hub
> and `proxy.golang.org` are unreachable:
>
> ```bash
> docker compose -f docker-compose.local.yml up -d
> ```

## Configuration

Configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_PATH` | SQLite database path | `./data/openvpn.db` |
| `PORT` | HTTP server port | `8080` |
| `EASYRSA_PATH` | EasyRSA installation path | `/etc/openvpn/easy-rsa` |
| `OPENVPN_PATH` | OpenVPN configuration path | `/etc/openvpn` |
| `CLIENTS_DIR` | Client configurations directory | `/etc/openvpn/clients` |
| `JWT_SECRET` | JWT signing secret (required) | - |
| `JWT_SECRET_FILE` | File containing JWT secret | - |
| `WORKER_COUNT` | Certificate worker count | `2` |
| `QUEUE_SIZE` | Certificate job queue size | `100` |

## CLI Usage

### User Management

```bash
# Create a new user
openvpn-mgmt user add john --email=john@example.com

# List all users
openvpn-mgmt user list

# List only active users
openvpn-mgmt user list --status=active

# Get user details
openvpn-mgmt user get john

# Delete/revoke a user
openvpn-mgmt user delete john
```

### Configuration Management

```bash
# Get user config (outputs to stdout)
openvpn-mgmt config get john > john.ovpn
```

### Database Management

```bash
# Backup database
openvpn-mgmt backup /backup/openvpn-$(date +%Y%m%d).db

# Restore database
openvpn-mgmt restore /backup/openvpn-20240101.db
```

### Server Management

```bash
# Start HTTP server
openvpn-mgmt serve

# Start on specific port
openvpn-mgmt serve --port=9090
```

## API Endpoints

### Users

- `GET /api/users` - List all users
- `POST /api/users` - Create a new user
- `DELETE /api/users/:name` - Delete a user
- `GET /api/users/:name/config` - Get user config

### Status

- `GET /api/status` - Get server status
- `GET /api/connected` - Get connected users

### Health

- `GET /health` - Health check

## Development

### Building

```bash
# Build binary
go build -o openvpn-mgmt ./cmd/server

# Run tests
go test ./...

# Run with race detector
go run -race ./cmd/server serve
```

### Project Structure

```
.
├── cmd/
│   └── server/           # Main application
│       ├── main.go       # Entry point
│       ├── cli.go        # CLI commands
│       └── server.go     # HTTP server
├── internal/
│   ├── auth/             # JWT authentication
│   ├── config/           # Configuration
│   ├── db/               # Database connection
│   ├── models/           # Data models
│   ├── repository/       # Data access layer
│   └── service/          # Business logic
├── api/
│   └── handlers.go       # HTTP handlers
├── scripts/
│   └── openvpn-mgmt.service  # Systemd unit
├── dashboard/            # Web dashboard (if any)
├── go.mod
├── go.sum
└── README.md
```

## License

MIT License
