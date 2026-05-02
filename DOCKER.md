# OpenVPN Management Service - Docker Compose Deployment

## Quick Start

```bash
# 1. Set JWT secret
export JWT_SECRET=$(openssl rand -base64 32)

# 2. Create data directory
mkdir -p data

# 3. Start the service
docker-compose up -d

# 4. Check logs
docker-compose logs -f openvpn-mgmt
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `DB_PATH` | /data/openvpn.db | SQLite database path |
| `JWT_SECRET` | required | JWT signing secret |
| `EASYRSA_PATH` | /etc/openvpn/easy-rsa | easy-rsa directory |
| `OPENVPN_PATH` | /etc/openvpn | OpenVPN config directory |
| `CLIENTS_DIR` | /etc/openvpn/clients | Client configs output |

### Volumes

The container requires access to:
1. `./data` - SQLite database persistence
2. `/etc/openvpn` - OpenVPN configuration (read-only)
3. `/etc/openvpn/easy-rsa` - Certificate generation (write)
4. `/etc/openvpn/clients` - Client config output (write)

## Production Deployment

### With HTTPS (Nginx)

```bash
# Generate self-signed certificate (for testing)
mkdir -p ssl
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout ssl/key.pem -out ssl/cert.pem \
  -subj "/C=US/ST=State/L=City/O=Org/CN=localhost"

# Start with nginx
docker-compose --profile with-nginx up -d
```

### Using Existing OpenVPN Server

```yaml
volumes:
  - /path/to/your/openvpn:/etc/openvpn:ro
  - /path/to/your/easy-rsa:/etc/openvpn/easy-rsa
  - /path/to/your/clients:/etc/openvpn/clients
```

## Management

```bash
# View logs
docker-compose logs -f

# Restart service
docker-compose restart openvpn-mgmt

# Stop everything
docker-compose down

# Update and rebuild
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

## API Endpoints

- `GET /health` - Health check
- `GET /api/users` - List users
- `POST /api/users` - Create user
- `GET /api/users/:id` - Get user
- `DELETE /api/users/:id` - Delete user
- `GET /api/users/:id/config` - Download .ovpn config
- `GET /api/status` - Server status
