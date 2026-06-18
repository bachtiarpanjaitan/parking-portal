# Photo Storage Module

## Purpose

The Parking Violation Portal requires officers to attach a photo when creating a violation.

This document defines how photos are uploaded, stored, and referenced by violations.

The implementation is intentionally simple and suitable for the assignment scope.

---

# Design Decision

## ADR Reference

See:

ADR-008 — Local File Storage

The system uses local filesystem storage.

Cloud object storage solutions such as:

* AWS S3
* Google Cloud Storage
* Azure Blob Storage
* MinIO

are intentionally excluded from the initial implementation.

The goal of this assignment is to focus on violation processing and rule versioning rather than storage infrastructure.

---

# Architecture

```text
Officer
   |
Upload Photo
   |
API Gateway
   |
Violation Service
   |
Local Filesystem
   |
Store Photo URL
   |
Create Violation
```

---

# Storage Strategy

Uploaded files are stored on the local filesystem.

Directory structure:

```text
storage/
└── violations/
    ├── c8d9a2c6.jpg
    ├── b7f3e1c6.png
    └── a1d4e716.jpeg
```

The storage directory must be created automatically if it does not exist.

---

# Database Storage

The system stores only the photo URL.

Violation table:

```sql
photo_url TEXT NOT NULL
```

Example:

```text
/uploads/violations/c8d9a2c6.jpg
```

Binary image data is never stored in PostgreSQL.

---

# Upload Flow

```text
Officer
    |
Select Photo
    |
POST /uploads/violations
    |
Validate File
    |
Store File
    |
Generate URL
    |
Return URL
    |
POST /violations
```

---

# Upload Endpoint

## POST /api/v1/uploads/violations

Content-Type:

```text
multipart/form-data
```

Request:

```text
file=<image>
```

Response:

```json
{
  "success": true,
  "data": {
    "file_name": "550e8400-e29b-41d4-a716-446655440000.jpg",
    "photo_url": "/uploads/violations/550e8400-e29b-41d4-a716-446655440000.jpg"
  }
}
```

---

# Static File Serving

Uploaded files are served as static files.

Example configuration:

```go
router.Static(
    "/uploads",
    "./storage"
)
```

Example:

File:

```text
storage/violations/550e8400-e29b-41d4-a716-446655440000.jpg
```

Accessible at:

```text
http://localhost:8080/uploads/violations/550e8400-e29b-41d4-a716-446655440000.jpg
```

---

# Supported Formats

Allowed formats:

```text
jpg
jpeg
png
webp
```

Rejected formats:

```text
gif
svg
pdf
zip
exe
```

---

# Validation Rules

Required validations:

* file must exist
* file size must be greater than zero
* file type must be supported
* file size must not exceed maximum limit

Maximum file size:

```text
5 MB
```

---

# Naming Strategy

Original filenames must not be used.

The system generates unique filenames using UUID.

Example:

```text
550e8400-e29b-41d4-a716-446655440000.jpg
```

Benefits:

* prevents filename collisions
* avoids special character issues
* improves security

---

# Frontend Workflow

Step 1

User selects an image.

Step 2

Frontend uploads image.

```http
POST /api/v1/uploads/violations
```

Step 3

Backend returns:

```json
{
  "photo_url": "/uploads/violations/file.jpg"
}
```

Step 4

Frontend submits violation.

```json
{
  "license_plate": "B1234XYZ",
  "violation_type": "no_parking_zone",
  "location": "Jakarta",
  "violation_timestamp": "2026-01-01T10:00:00Z",
  "photo_url": "/uploads/violations/file.jpg"
}
```

---

# Docker Configuration

To preserve uploaded files between container restarts, the storage directory should be mounted as a volume.

Example:

```yaml
violation-service:
  volumes:
    - violation_uploads:/app/storage

volumes:
  violation_uploads:
```

Development alternative:

```yaml
violation-service:
  volumes:
    - ./storage:/app/storage
```

---

# Error Responses

Unsupported file type:

```json
{
  "success": false,
  "message": "Unsupported file type"
}
```

File too large:

```json
{
  "success": false,
  "message": "File size exceeds maximum limit"
}
```

Upload failed:

```json
{
  "success": false,
  "message": "Upload failed"
}
```

---

# Security Considerations

Required:

* validate MIME type
* validate extension
* generate random filenames
* limit upload size

Not required for this assignment:

* virus scanning
* image optimization
* CDN integration
* signed URLs
* image watermarking

---

# Future Improvements

Possible future enhancements:

* AWS S3 integration
* MinIO integration
* Image compression
* Thumbnail generation
* Signed URLs
* CDN delivery
* Object lifecycle management

These features are intentionally out of scope for the current assignment.

---

# Assignment Scope

Required:

* upload image
* store image locally
* return image URL
* save image URL in violation record
* serve image through HTTP

Optional:

* image processing
* cloud storage
* media management

Local filesystem storage is sufficient for the assignment requirements.
