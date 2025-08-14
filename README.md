# IPInfo API

An open source IP geolocation API that provides detailed information about IP addresses, including geographic location, ISP details, and network information. Compatible with ipinfo.io response format.

## Features

- 🌍 **Geographic Location**: City, region, country, postal code, timezone
- 🏢 **ISP Information**: Organization name with human-readable mappings
- 🔍 **Hostname Resolution**: Reverse DNS lookup with timeout
- 🚫 **Bogon Detection**: Identifies private, reserved, and special-use IP addresses
- 🔄 **Auto-updating Databases**: Hourly checks for MaxMind GeoLite2 database updates
- ⚡ **Fast Response**: Optimized for speed with concurrent operations

## Quick Start

1. **Clone and run:**
   ```bash
   git clone https://github.com/tristandevs/ipinfo-api
   cd ipinfo-api
   go run main.go
   ```

2. **Server starts on port 8080**

## API Usage

### Get your own IP information
```bash
curl http://localhost:8080/
```

### Get information for a specific IP
```bash
curl http://localhost:8080/208.67.222.222
```

## Response Format

### Public IP Response
```json
{
  "ip": "208.67.222.222",
  "city": "San Francisco",
  "region": "California",
  "country": "US",
  "loc": "37.764200,-122.399300",
  "org": "AS36692 CISCO-UMBRELLA",
  "postal": "94107",
  "timezone": "America/Los_Angeles"
}
```

### Private IP Response
```json
{
  "ip": "192.168.1.1",
  "bogon": true
}
```

### Error Response
```json
{
  "status": 404,
  "error": {
    "title": "Wrong ip",
    "message": "Please provide a valid IP address"
  }
}
```

## Supported Headers

The API automatically detects client IP from these headers:
- `X-Forwarded-For`
- `X-Real-IP`
- `CF-Connecting-IP` (Cloudflare)

## Database Updates

The service automatically:
- Downloads GeoLite2 databases on first run
- Checks for updates every hour
- Uses atomic file replacement to prevent corruption
- Compares `Last-Modified` headers for efficient updates

Database sources:
- GeoLite2-ASN.mmdb
- GeoLite2-City.mmdb  
- GeoLite2-Country.mmdb

## Configuration

### Timeouts
- DNS lookup: 40ms context timeout, 50ms total timeout
- Database update checks: 10s
- Database downloads: 60s

### ISP Mappings
The service includes human-readable mappings for major ISPs including:
- Cloud providers (AWS, Google Cloud, Microsoft Azure)
- CDNs (Cloudflare, Fastly)
- Major ISPs (Comcast, Verizon, AT&T, Charter)
- Hosting providers (DigitalOcean, Linode, Rackspace)

## Requirements

- Go 1.19+
- Internet connection for database updates
- ~100MB disk space for databases

## Contributing

Contributions welcome! Please feel free to submit issues and pull requests.
