# Co-ATC Docker Setup

This guide provides instructions for deploying Co-ATC using Docker and Docker Compose.

## Overview

Co-ATC is an AI-enhanced aircraft monitoring system that provides:

- Real-time aircraft tracking with interactive maps
- Audio streaming from multiple ATC frequencies
- AI transcription of radio communications (requires OpenAI API)
- AI voice assistant for ATC queries
- Weather integration (METAR/TAF/NOTAMs)
- WebSocket updates for real-time data

## Quick Start

### Option 1: Automated Setup (Recommended)

1. **Run the setup script:**
   ```bash
   cd docker
   ./start.sh
   ```

2. **The script will:**
   - Check for required files and dependencies
   - Create a configuration file from the example template
   - Build and start the container
   - Open the web interface in your browser

3. **Configure your settings:**
   - When prompted, edit `config.toml` with your settings (see Configuration section below)
   - Run the script again to start the system

### Option 2: Manual Setup

1. **Create configuration file:**
   ```bash
   cd docker
   cp ../configs/config.toml.example config.toml
   ```

2. **Edit configuration file:**
   ```bash
   nano config.toml
   ```

3. **Launch the system:**
   ```bash
   docker-compose up -d
   ```

4. **Access the interface:**
   - Open your browser to: `http://localhost:8000`

## Prerequisites

- Docker & Docker Compose installed
- ADS-B data source (local tar1090 or external API)
- OpenAI API key (optional, for AI features)
- FFmpeg support is included in the container

## Configuration

The configuration file is created from `/configs/config.toml.example`. You must customize the following settings for your environment:

### Required Settings

#### 1. ADS-B Data Source
Configure your aircraft data source:

```toml
[adsb]
# For local tar1090 server (recommended)
source_type = "local"
local_source_url = "http://your-tar1090-server:8080/tar1090/data/aircraft.json"

# OR for external API (requires API key)
# source_type = "external"
# external_source_url = "https://adsbexchange-com1.p.rapidapi.com/v2/lat/%f/lon/%f/dist/%.0f/"
# api_key = "your-api-key-here"
```

#### 2. Station Location
Set your monitoring station coordinates:

```toml
[station]
latitude = 43.6777      # Your latitude
longitude = -79.6248    # Your longitude
elevation_feet = 569    # Your elevation in feet
airport_code = "CYYZ"   # Your airport ICAO code
```

#### 3. Radio Frequencies
Configure ATC frequencies to monitor:

```toml
[[frequencies.sources]]
id = "tower"
airport = "CYYZ"
name = "Tower"
frequency_mhz = 118.700
url = "https://your-audio-stream-url"
order = 1
transcribe_audio = true
```

### Optional Settings

#### 4. OpenAI API Integration
Enable AI features with your OpenAI API key:

```toml
[transcription]
openai_api_key = "sk-your-key-here"

[atc_chat]
openai_api_key = "sk-your-key-here"
```

#### 5. Server Configuration
The default server settings work with Docker:

```toml
[server]
host = "0.0.0.0"        # Bind to all interfaces
port = 8080             # Internal port (mapped to 8000 externally)
```

### Configuration Notes

- The container runs on internal port 8080, mapped to external port 8000
- Database files are stored in the persistent `data` volume
- Configuration examples are provided for Toronto Pearson (CYYZ) airport
- Audio streams require valid URLs (LiveATC, local SRT streams, etc.)
- AI features require valid OpenAI API keys with appropriate permissions



## Data Persistence

The setup includes persistent volumes for:

- **`co-atc-data`** - SQLite databases and application data
- **`co-atc-logs`** - Application logs

**File mounts:**
- **`../assets`** - Contains AI prompts, airlines, airports, and runways data
- **`../www`** - Web interface files (HTML, CSS, JS)
- **`./config.toml`** - Your configuration file

Data survives container restarts and updates.

## Deployment Commands

### Recommended: Use the automated script
```bash
cd docker
./start.sh
```

### Manual commands

**Start the system:**
```bash
cd docker
docker-compose up -d
```

**View logs (real-time):**
```bash
docker-compose logs -f co-atc
```

**View recent logs:**
```bash
docker-compose logs --tail 50 co-atc
```

**Check system status:**
```bash
docker-compose ps
curl -I http://localhost:8000
```

**Restart the system:**
```bash
docker-compose restart co-atc
```

**Stop the system:**
```bash
docker-compose down
```

**Update and rebuild:**
```bash
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

**Clean everything (⚠️ DESTROYS DATA):**
```bash
docker-compose down -v
docker system prune -a
```

### Quick Status Check
```bash
# Check if everything is running
docker-compose ps
docker-compose logs co-atc | grep "Starting HTTP server"
curl -I http://localhost:8000
```

## 🌐 Network Configuration

The system exposes multiple ports:
- **8000** - Main web interface
- **8001-8004** - Additional interfaces

### Behind a Reverse Proxy

Example Nginx config:
```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Troubleshooting

### Common Issue: "Invalid configuration: frequency #1: URL is required"
This error indicates the configuration file isn't being loaded correctly or contains invalid frequency configurations.

**Solution:**
```bash
# Check if config file exists
ls -la docker/config.toml

# Check docker-compose.yml volume mount (should be exactly this):
grep -A 1 "Configuration" docker/docker-compose.yml
# Should show: - ./config.toml:/app/configs/config.toml:ro

# Verify frequency configuration in config.toml
grep -A 10 "frequencies.sources" docker/config.toml
```

### Common Issue: Container won't start
```bash
# Check Docker is running
docker info

# Check logs for specific error
docker-compose logs co-atc

# Verify configuration syntax
docker-compose config

# Force rebuild if needed
docker-compose build --no-cache co-atc
```

### Common Issue: Web interface not accessible
The application runs on port 8080 inside the container, mapped to port 8000 externally.

**Solution:**
```bash
# Check if container is running
docker-compose ps

# Test internal port first
docker exec co-atc wget -qO- http://localhost:8080 | head -10

# Test external port mapping
curl -I http://localhost:8000
```

### Common Issue: No aircraft data
1. Check your ADS-B source URL in `config.toml`
2. Verify network connectivity: `ping your-tar1090-server`
3. Check if tar1090 is running and accessible
4. Look for ADS-B errors in logs: `docker-compose logs co-atc | grep adsb`

### Common Issue: Missing assets/prompts errors
1. Ensure assets directory exists: `ls -la ../assets/`
2. Check required files exist:
   ```bash
   ls -la ../assets/*.json
   ls -la ../assets/*.txt
   ```
3. Verify volume mount: `docker exec co-atc ls -la assets/`

### Common Issue: Audio/frequency issues
1. Verify FFmpeg is working: `docker exec co-atc ffmpeg -version`
2. Check audio stream URLs are accessible from container:
   ```bash
   docker exec co-atc wget --spider https://s1-bos.liveatc.net/cyyz7
   ```
3. Review frequency configuration in `config.toml`
4. Check for frequency errors: `docker-compose logs co-atc | grep freq`

### Common Issue: AI features not working
1. Verify OpenAI API key is set in `config.toml`
2. Check API key permissions and billing
3. Monitor logs for API errors: `docker-compose logs co-atc | grep openai`

### Common Issue: Health check failing
```bash
# Check container health
docker-compose ps

# Manual health check
docker exec co-atc wget --no-verbose --tries=1 --spider http://localhost:8080/

# Check if port is accessible
curl -I http://localhost:8000
```

## 📊 Monitoring

### Resource usage:
```bash
docker stats co-atc
```

### Database size:
```bash
docker exec co-atc ls -lh data/
```

### Application status:
```bash
curl http://localhost:8000/
```

## 🔒 Security Considerations

**⚠️ IMPORTANT SECURITY WARNING:**

This application has **NO AUTHENTICATION** and should **NEVER** be exposed to the internet!

### Security measures:
- Only bind to localhost (`127.0.0.1:8000`)
- Use a reverse proxy with authentication
- Keep containers updated
- Monitor logs for suspicious activity

### For production use:
1. Set up proper authentication
2. Use HTTPS/TLS encryption
3. Implement rate limiting
4. Regular security updates
5. Network segmentation

## 🐛 Common Issues

### "Permission denied" errors:
```bash
# Fix volume permissions
docker-compose down
sudo chown -R 1000:1000 /var/lib/docker/volumes/docker_co-atc-data/
docker-compose up -d
```

### "Config file not found":
```bash
# Ensure config file exists
ls -la docker/config.toml.local
# Check volume mount in docker-compose.yml
```

### High CPU usage:
- Reduce `fetch_interval_seconds` in config
- Disable unnecessary frequencies
- Check for infinite loops in logs

### Memory issues:
- Increase container memory limits
- Check for memory leaks in logs
- Restart container periodically

## 🚀 Performance Tuning

### Resource limits:
```yaml
deploy:
  resources:
    limits:
      memory: 1G        # Increase if needed
      cpus: '2.0'       # Increase for better performance
```

### Database optimization:
- Regular database cleanup
- Monitor database size
- Consider external database for high-volume deployments

## 📚 Additional Resources

- [Main Project README](../README.md)
- [API Documentation](../docs/api_spec.md)
- [Technical Documentation](../docs/technical_docs.md)
- [Project Progress](../docs/project_progress.md)

## Support

This is a development/hobby project. For issues:
1. Check the logs first
2. Verify your configuration
3. Review this README
4. Check the main project documentation

## Additional Resources

- [Main Project README](../README.md)
- [API Documentation](../docs/api_spec.md)
- [Technical Documentation](../docs/technical_docs.md)
- [Project Progress](../docs/project_progress.md) 