# Docker Compose Configurations

Two deployment configurations for myCart application.

## Quick Reference

```bash
# XMPP server deployment (production)
docker-compose -f docker/docker-compose.yml up -d

# Production with SSL proxy (SQLite or PostgreSQL)
cd docker && docker-compose -f docker-compose.onsite.yml up -d
```

---

## 1. docker-compose.yml - XMPP Server (Production)

Production XMPP deployment with Prosody server and myCart integration.

**Services:**
- prosody-modules-init - One-time module setup
- prosody-config-init - Configuration renderer
- prosody - XMPP server
- xmpp-proxy-stack - myCart + XMPP proxy + fail2ban

**Documentation:** See [xmpp-proxy-stack/README.md](../xmpp-proxy-stack/README.md) for detailed setup instructions.

**Quick Start:**
```bash
# Create data directories
sudo mkdir -p /srv/data/{prosody,certs,logs,fail2ban,mycart/{lc_base,lc_uploads,lc_digitals}}
sudo chown -R 1000:1000 /srv/data

# Configure .env (see xmpp-proxy-stack/README.md for all variables)
cat > .env << EOF
XMPP_DOMAIN=chat.example.com
MYCART_DOMAIN=chat.example.com
XMPP_ADMIN=admin@chat.example.com
XMPP_PROXY_PROSODY_C2S=127.0.0.1:15222
XMPP_PROXY_PROSODY_S2S=127.0.0.1:15269
EOF

# Start services
docker-compose -f docker/docker-compose.yml up -d
```

---

## 2. docker-compose.onsite.yml - Production with SSL

Production deployment with automatic SSL certificates and optional PostgreSQL.

**Services:**
- nginx-proxy - Reverse proxy
- acme-companion - Let's Encrypt SSL automation
- mycart - Application
- postgres - Optional database (profile-based)

**Ports:**
- 80 - HTTP (redirects to HTTPS)
- 443 - HTTPS

**Prerequisites:**
1. Point domain DNS to server IP
2. Create `.env` file:

```bash
cd docker
cat > .env << EOF
DOMAIN=yourdomain.com
ADMIN_EMAIL=admin@yourdomain.com
EOF
```

**Usage:**

**SQLite mode (default):**
```bash
cd docker
docker-compose -f docker-compose.onsite.yml up -d
```

**PostgreSQL mode:**
```bash
cd docker

# Add to .env
cat >> .env << EOF
MYCART_DB_DRIVER=postgres
MYCART_DB_DSN=postgres://mycart:secure_password@postgres:5432/mycart?sslmode=disable
POSTGRES_PASSWORD=secure_password
POSTGRES_DB=mycart
POSTGRES_USER=mycart
EOF

# Start with postgres profile
docker-compose -f docker-compose.onsite.yml --profile postgres up -d
```

**Data:**
- `./lc_base/` - SQLite database (default)
- `./lc_pgdata/` - PostgreSQL data (if using postgres profile)
- `./lc_uploads/`, `./lc_digitals/`, `./site/` - Application data

**Access:** https://yourdomain.com

**Certificate Management:**
- Certificates auto-renew before expiration
- Check status: `docker logs nginx-proxy-acme`
- Manual renewal: `docker restart nginx-proxy-acme`

---

## Common Commands

**View logs:**
```bash
docker-compose -f docker/docker-compose.yml logs -f
docker logs mycart -f
```

**Stop services:**
```bash
docker-compose -f docker/docker-compose.yml down
```

**Restart service:**
```bash
docker-compose -f docker/docker-compose.yml restart mycart
```

**Execute commands:**
```bash
docker exec -it mycart sh
docker exec -it prosody prosodyctl status
```

**Backup database:**
```bash
# SQLite
cp ./lc_base/mycart.db ./lc_base/mycart.db.backup

# PostgreSQL
docker exec mycart-postgres pg_dump -U mycart mycart > backup.sql
```

**Restore database:**
```bash
# SQLite
cp ./lc_base/mycart.db.backup ./lc_base/mycart.db

# PostgreSQL
cat backup.sql | docker exec -i mycart-postgres psql -U mycart mycart
```

---

## Environment Variables

### docker-compose.yml
- `XMPP_DOMAIN` - XMPP domain (required)
- `MYCART_DOMAIN` - myCart domain, must match XMPP_DOMAIN (required)
- `XMPP_ADMIN` - Admin JID (required)
- `XMPP_PROXY_PROSODY_C2S` - Backend Prosody c2s port (required)
- `XMPP_PROXY_PROSODY_S2S` - Backend Prosody s2s port (required)
- `PROSODY_LOGLEVEL` - Log level (default: info)
- `PROSODY_RETENTION_DAYS` - Message retention (default: 90)
- See [xmpp-proxy-stack/README.md](../xmpp-proxy-stack/README.md) for complete list

### docker-compose.onsite.yml
- `DOMAIN` - Your domain (required)
- `ADMIN_EMAIL` - Email for SSL notifications (required)
- `MYCART_DB_DRIVER` - Database: empty (SQLite) or `postgres`
- `MYCART_DB_DSN` - PostgreSQL connection string
- `POSTGRES_PASSWORD` - PostgreSQL password (required for postgres profile)
- `POSTGRES_DB` - Database name (default: mycart)
- `POSTGRES_USER` - Database user (default: mycart)

---

## Troubleshooting

**Container won't start:**
```bash
docker logs mycart
docker-compose ps
```

**Port already in use:**
```bash
# Find process using port 8080
ss -tnlp | grep 8080
# Kill or change port
```

**Permission denied:**
```bash
sudo chown -R $USER:$USER ./lc_base ./lc_uploads ./lc_digitals
```

**SSL certificate issues:**
```bash
# Check acme-companion logs
docker logs nginx-proxy-acme

# Verify DNS
nslookup yourdomain.com

# Restart SSL companion
docker restart nginx-proxy-acme
```

**XMPP connection issues:**
```bash
# Check Prosody status
docker exec prosody prosodyctl status

# Check ports are listening
ss -tnlup | grep -E '5222|5269'

# Check configuration
docker exec prosody prosodyctl check
```

**PostgreSQL connection issues:**
```bash
# Test connection
docker exec mycart-postgres pg_isready -U mycart

# Check environment
docker exec mycart env | grep DB_
```

---

## Security Recommendations

1. **Change default passwords:**
   ```bash
   POSTGRES_PASSWORD=$(openssl rand -base64 32)
   ```

2. **Use .env files for secrets:**
   ```bash
   chmod 600 .env
   echo ".env" >> .gitignore
   ```

3. **Keep images updated:**
   ```bash
   docker-compose pull
   docker-compose up -d
   ```

4. **Regular backups:**
   Schedule automated daily backups of databases and data directories

5. **Monitor logs:**
   ```bash
   docker logs mycart --tail 100 -f
   ```
