# Security Configuration

Container Census supports optional HTTP Basic Authentication to secure access to the UI and telemetry collector.

## UI Server Authentication

The main Container Census server can be protected with HTTP Basic Authentication to restrict access to the web UI and API endpoints.

### Configuration via Environment Variables

Set the following environment variables to enable authentication:

```bash
AUTH_ENABLED=true
AUTH_USERNAME=your_username
AUTH_PASSWORD=your_secure_password
```

### Configuration via Config File

Add authentication settings to your `config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  auth:
    enabled: true
    username: your_username
    password: your_secure_password
```

### Docker Example

When running with Docker, pass the environment variables:

```bash
docker run -d \
  -e AUTH_ENABLED=true \
  -e AUTH_USERNAME=admin \
  -e AUTH_PASSWORD=your_secure_password \
  -p 8080:8080 \
  -v census-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  container-census:latest
```

### Docker Compose Example

```yaml
services:
  census:
    image: container-census:latest
    ports:
      - "8080:8080"
    volumes:
      - census-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - AUTH_ENABLED=true
      - AUTH_USERNAME=admin
      - AUTH_PASSWORD=your_secure_password
```

## Telemetry Collector Authentication

The telemetry collector service can also be protected with HTTP Basic Authentication.

### Configuration via Environment Variables

Set the following environment variables for the telemetry collector:

```bash
COLLECTOR_AUTH_ENABLED=true
COLLECTOR_AUTH_USERNAME=collector_user
COLLECTOR_AUTH_PASSWORD=collector_secure_password
```

### Docker Compose Example

```yaml
services:
  telemetry-collector:
    image: container-census-telemetry:latest
    ports:
      - "8081:8081"
    environment:
      - DATABASE_URL=postgres://user:password@postgres:5432/telemetry
      - COLLECTOR_AUTH_ENABLED=true
      - COLLECTOR_AUTH_USERNAME=collector_user
      - COLLECTOR_AUTH_PASSWORD=collector_secure_password
```

## Public Endpoints

Even when authentication is enabled, the following endpoints remain publicly accessible for monitoring purposes:

- `/api/health` - Health check endpoint for both UI server and telemetry collector

## Security Considerations

1. **Use Strong Passwords**: Always use strong, randomly generated passwords for authentication.

2. **HTTPS Recommended**: HTTP Basic Authentication transmits credentials in base64 encoding (not encrypted). Always use HTTPS/TLS in production by placing Container Census behind a reverse proxy like nginx or Traefik.

3. **Environment Variables vs Config File**:
   - Environment variables override config file settings
   - Use environment variables for sensitive credentials to avoid storing them in config files
   - Config files are useful for non-sensitive defaults

4. **Reverse Proxy Configuration**: For production deployments, use a reverse proxy with TLS:

   ```nginx
   server {
       listen 443 ssl http2;
       server_name census.example.com;

       ssl_certificate /path/to/cert.pem;
       ssl_certificate_key /path/to/key.pem;

       location / {
           proxy_pass http://localhost:8080;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
       }
   }
   ```

5. **Default Behavior**: If authentication is not configured, the services are publicly accessible. Make sure to enable authentication if exposing services to untrusted networks.

## Updating Telemetry Submission

If you've enabled authentication on the telemetry collector, you'll need to update the telemetry endpoint configuration in your Container Census instances to include credentials:

```yaml
telemetry:
  enabled: true
  endpoints:
    - name: my-collector
      url: https://username:password@collector.example.com/api/ingest
      enabled: true
```

Or use basic auth in the URL format: `https://username:password@host:port/api/ingest`
