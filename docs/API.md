# API Documentation

This document describes the API endpoints available in the AI Code Template Go application.

## Base URL

```
http://localhost:8080
```

## Authentication

Currently, this template does not implement authentication. In a production application, you should implement proper authentication and authorization.

## Endpoints

### Health Check

**GET** `/health`

Returns the health status of the service.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Version Information

**GET** `/version`

Returns version information about the service.

**Response:**
```json
{
  "version": "1.0.0",
  "commit": "abc123def456",
  "date": "2024-01-15T10:30:00Z",
  "builtBy": "goreleaser"
}
```

### Hello Endpoint

**GET** `/api/v1/hello`

Returns a greeting message.

**Response:**
```json
{
  "success": true,
  "message": "Hello from AI Code Template Go!",
  "data": {
    "service": "ai-code-template-go",
    "time": "2024-01-15T10:30:00Z"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "path": "/api/v1/hello"
}
```

### Status Endpoint

**GET** `/api/v1/status`

Returns the current status of the service.

**Response:**
```json
{
  "success": true,
  "message": "Service is running",
  "data": {
    "status": "healthy",
    "uptime": "running",
    "timestamp": "2024-01-15T10:30:00Z"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "path": "/api/v1/status"
}
```

## Error Responses

All endpoints return consistent error responses in the following format:

```json
{
  "success": false,
  "error": "Not Found",
  "message": "Endpoint not found",
  "timestamp": "2024-01-15T10:30:00Z",
  "path": "/invalid-endpoint"
}
```

## HTTP Status Codes

- `200 OK` - Request successful
- `404 Not Found` - Endpoint not found
- `405 Method Not Allowed` - HTTP method not supported
- `500 Internal Server Error` - Server error

## Rate Limiting

Currently, this template does not implement rate limiting. In a production application, you should implement appropriate rate limiting to prevent abuse.

## CORS

Currently, this template does not implement CORS. In a production application, you should configure CORS appropriately for your use case.

## Examples

### Using curl

```bash
# Health check
curl http://localhost:8080/health

# Get version
curl http://localhost:8080/version

# Hello endpoint
curl http://localhost:8080/api/v1/hello

# Status endpoint
curl http://localhost:8080/api/v1/status
```

### Using JavaScript

```javascript
// Health check
fetch('http://localhost:8080/health')
  .then(response => response.json())
  .then(data => console.log(data));

// Hello endpoint
fetch('http://localhost:8080/api/v1/hello')
  .then(response => response.json())
  .then(data => console.log(data));
```

## Future Enhancements

This template can be extended with:

- Authentication and authorization
- Database integration
- Redis caching
- Metrics and monitoring
- Rate limiting
- CORS configuration
- API versioning
- Request/response logging
- Input validation
- Error handling middleware
