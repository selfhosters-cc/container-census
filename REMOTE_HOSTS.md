# Remote Host Configuration Guide

Container Census supports scanning Docker containers across multiple hosts. This guide explains your options for connecting to remote Docker hosts and how to configure them.

## Connection Methods

Container Census supports three methods for connecting to Docker hosts:

1. **Unix Socket** - Local Docker daemon (default)
2. **TCP** - Remote Docker daemon via TCP (with or without TLS)
3. **SSH** - Remote Docker daemon via SSH tunnel

---

## Method 1: Unix Socket (Local)

**Best for:** Scanning the local Docker daemon where Container Census is running.

### Configuration

```yaml
hosts:
  - name: local
    address: unix:///var/run/docker.sock
    description: Local Docker daemon
```

### Requirements

- Container Census must have access to the Docker socket
- When running in Docker, mount the socket: `-v /var/run/docker.sock:/var/run/docker.sock`
- The container user must have permission to access the socket (see [DOCKER_PERMISSIONS.md](DOCKER_PERMISSIONS.md))

---

## Method 2: TCP Connection

**Best for:** Remote Docker hosts on your network where you control the Docker daemon configuration.

### Option A: Unencrypted TCP (Not Recommended for Production)

⚠️ **Warning:** This exposes your Docker daemon without authentication. Only use on trusted networks.

#### Docker Host Setup

1. **Configure Docker daemon to listen on TCP:**

   Edit `/etc/docker/daemon.json`:
   ```json
   {
     "hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2375"]
   }
   ```

2. **If using systemd, update the service file:**

   Create/edit `/etc/systemd/system/docker.service.d/override.conf`:
   ```ini
   [Service]
   ExecStart=
   ExecStart=/usr/bin/dockerd
   ```

3. **Restart Docker:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart docker
   ```

4. **Verify it's listening:**
   ```bash
   curl http://localhost:2375/version
   ```

#### Container Census Configuration

```yaml
hosts:
  - name: remote-host
    address: tcp://192.168.1.100:2375
    description: Remote Docker host (unencrypted)
```

### Option B: TCP with TLS (Recommended)

🔒 **Recommended:** Secure your Docker daemon with TLS certificates.

#### Docker Host Setup

1. **Generate certificates:**

   ```bash
   # Create a CA
   openssl genrsa -aes256 -out ca-key.pem 4096
   openssl req -new -x509 -days 365 -key ca-key.pem -sha256 -out ca.pem

   # Create server key and certificate
   openssl genrsa -out server-key.pem 4096
   openssl req -subj "/CN=your-host.example.com" -sha256 -new -key server-key.pem -out server.csr

   echo subjectAltName = DNS:your-host.example.com,IP:192.168.1.100,IP:127.0.0.1 >> extfile.cnf
   echo extendedKeyUsage = serverAuth >> extfile.cnf

   openssl x509 -req -days 365 -sha256 -in server.csr -CA ca.pem -CAkey ca-key.pem \
     -CAcreateserial -out server-cert.pem -extfile extfile.cnf

   # Create client key and certificate
   openssl genrsa -out key.pem 4096
   openssl req -subj '/CN=client' -new -key key.pem -out client.csr

   echo extendedKeyUsage = clientAuth > extfile-client.cnf

   openssl x509 -req -days 365 -sha256 -in client.csr -CA ca.pem -CAkey ca-key.pem \
     -CAcreateserial -out cert.pem -extfile extfile-client.cnf

   # Set permissions
   chmod -v 0400 ca-key.pem key.pem server-key.pem
   chmod -v 0444 ca.pem server-cert.pem cert.pem
   ```

2. **Configure Docker daemon:**

   Edit `/etc/docker/daemon.json`:
   ```json
   {
     "hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2376"],
     "tls": true,
     "tlscacert": "/etc/docker/certs/ca.pem",
     "tlscert": "/etc/docker/certs/server-cert.pem",
     "tlskey": "/etc/docker/certs/server-key.pem",
     "tlsverify": true
   }
   ```

3. **Copy certificates to `/etc/docker/certs/`**

4. **Restart Docker:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart docker
   ```

#### Container Census Configuration

> **Note:** TLS certificate support for TCP connections is not yet implemented in Container Census. This is a planned feature. For now, use SSH method for secure remote connections.

---

## Method 3: SSH Tunnel (Recommended for Remote Hosts)

**Best for:** Secure connections to remote Docker hosts without exposing the Docker daemon.

### Advantages

- ✅ Secure: Uses SSH authentication and encryption
- ✅ No Docker daemon reconfiguration needed
- ✅ Works through firewalls (only SSH port needs to be open)
- ✅ Leverages existing SSH access controls

### Docker Host Setup

1. **Ensure SSH server is running:**
   ```bash
   sudo systemctl status ssh
   ```

2. **Create a dedicated user (recommended):**
   ```bash
   sudo useradd -r -s /bin/bash -m docker-census
   sudo usermod -aG docker docker-census
   ```

3. **Set up SSH key authentication:**

   On the Container Census host:
   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/docker-census -C "docker-census"
   ```

   Copy the public key to the remote host:
   ```bash
   ssh-copy-id -i ~/.ssh/docker-census.pub docker-census@192.168.1.100
   ```

4. **Test the connection:**
   ```bash
   ssh -i ~/.ssh/docker-census docker-census@192.168.1.100 docker ps
   ```

### Container Census Configuration

```yaml
hosts:
  - name: production-server
    address: ssh://docker-census@192.168.1.100
    description: Production Docker host via SSH
```

### SSH with Custom Port

If your SSH server runs on a non-standard port:

```yaml
hosts:
  - name: production-server
    address: ssh://docker-census@192.168.1.100:2222
    description: Production Docker host (SSH port 2222)
```

### SSH Key Authentication in Docker

When running Container Census in Docker, you need to mount your SSH key:

```bash
docker run -d \
  --name container-census \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/.ssh/docker-census:/root/.ssh/id_rsa:ro \
  -v ./config/config.yaml:/app/config/config.yaml \
  -v ./data:/app/data \
  container-census:latest
```

> **Note:** Full SSH support is currently in development. The Docker SDK supports SSH connections, but key management and host verification need additional configuration.

---

## Configuration File Reference

### Complete Example

```yaml
# Container Census Configuration

database:
  path: ./data/census.db

server:
  host: 0.0.0.0
  port: 8080

scanner:
  interval_seconds: 300  # Scan every 5 minutes
  timeout_seconds: 30    # Timeout for each scan operation

hosts:
  # Local Docker daemon
  - name: local
    address: unix:///var/run/docker.sock
    description: Local Docker daemon

  # Remote host via TCP (unencrypted - testing only)
  - name: dev-server
    address: tcp://192.168.1.50:2375
    description: Development server

  # Remote host via TCP with TLS (when supported)
  - name: staging-server
    address: tcp://192.168.1.100:2376
    description: Staging server (TLS)

  # Remote host via SSH (recommended)
  - name: production-server
    address: ssh://docker-census@prod.example.com
    description: Production Docker host

  # SSH with custom port
  - name: backup-server
    address: ssh://docker-census@backup.example.com:2222
    description: Backup server (custom SSH port)
```

### Reloading Configuration

After editing `config.yaml`, you have two options:

1. **Use the Web UI:** Click the "Reload Config" button in the header
2. **Use the API:** `curl -X POST http://localhost:8080/api/config/reload`
3. **Restart the container:** `docker restart container-census`

The reload feature will add new hosts and update existing ones without restarting the service.

---

## Troubleshooting

### Connection Issues

#### "Cannot connect to the Docker daemon"

**TCP connections:**
- Verify the Docker daemon is listening: `netstat -tlnp | grep 2375`
- Check firewall rules: `sudo ufw status`
- Test from Census host: `curl http://REMOTE_HOST:2375/version`

**SSH connections:**
- Verify SSH access: `ssh user@remote-host docker ps`
- Check SSH key permissions: `chmod 600 ~/.ssh/id_rsa`
- Ensure user is in docker group: `groups docker-census`

#### "Permission denied"

- Ensure the user has Docker socket access: `sudo usermod -aG docker USERNAME`
- Verify group membership: `groups USERNAME`
- Log out and back in for group changes to take effect

#### "Connection timeout"

- Check network connectivity: `ping remote-host`
- Verify firewall allows connections on the Docker port
- For SSH: Ensure SSH port is open and accessible

### Viewing Scan Results

Check the "Scan Results" tab in the web UI or view logs:

```bash
docker logs container-census
```

Look for messages like:
```
Scan completed for host production-server: found 15 containers
Scan failed for host dev-server: connection refused
```

### Enabling/Disabling Hosts

Hosts can be enabled/disabled through the database. This feature will be added to the web UI in a future update.

---

## Security Best Practices

1. **Never use unencrypted TCP in production**
   - Use SSH tunneling or TLS encryption

2. **Use dedicated service accounts**
   - Create separate users for Container Census
   - Grant only necessary Docker permissions

3. **Restrict network access**
   - Use firewall rules to limit who can access Docker daemons
   - Consider VPN or SSH tunneling for internet-accessible hosts

4. **Rotate credentials regularly**
   - Update SSH keys periodically
   - Use certificate expiration for TLS

5. **Monitor access logs**
   - Check Docker daemon logs for suspicious activity
   - Review SSH access logs: `/var/log/auth.log`

6. **Use read-only access when possible**
   - Container Census only needs read access to scan
   - Container management features require write access

---

## Feature Support Matrix

| Connection Type | Status | Authentication | Encryption | Use Case |
|----------------|--------|----------------|------------|----------|
| Unix Socket | ✅ Full | File permissions | N/A | Local daemon |
| TCP (unencrypted) | ✅ Full | None | ❌ None | Testing only |
| TCP with TLS | 🚧 Planned | TLS certs | ✅ TLS | Production remote |
| SSH | 🚧 Partial | SSH keys | ✅ SSH | Recommended remote |

**Legend:**
- ✅ Full support
- 🚧 In development
- ❌ Not supported/not recommended

---

## Need Help?

- **Documentation:** [README.md](README.md)
- **API Reference:** [API.md](API.md)
- **Docker Permissions:** [DOCKER_PERMISSIONS.md](DOCKER_PERMISSIONS.md)
- **Issues:** [GitHub Issues](https://github.com/yourusername/container-census/issues)
