# FileBox — Overall Project Specification (v1.0)

## 1) Goal & Summary
Build a self-hosted, multi-user file sharing & collaboration web app.  
Users can:
- Register & log in
- Upload files
- Manage metadata/tags
- List & search files
- Share time-limited links

The system is **stateless at the API layer** so Nginx can round-robin across multiple backend replicas.

**Key Deliverable:**  
A Docker-based deployment with official images where possible, plus a single custom backend (Gin). Runs locally with Docker Compose; easy to extend to Swarm/Kubernetes later.

---

## 2) Success Criteria (for grading/demo)
- `docker compose up -d --build` brings the full stack up
- Nginx reverse proxies and **round-robins** requests across ≥ 2 API replicas
- Uploads use **pre-signed URLs** (client → MinIO directly)
- Core flows work: **register → login → upload → list → share → download**
- Clear `README` + sample curl/scripts + demo account provided
- **Stateless behavior verified**: kill one API container → session still works via other replicas

---

## 3) Scope

### In Scope (MVP)
- Auth (email + password, JWT access token, short TTL)
- File upload via pre-signed PUT to MinIO; file metadata in Postgres
- Listing, filtering, tag search; per-user data isolation
- Share links (time-limited tokens)
- Activity log (minimal events)
- Rate limiting (best-effort) at Nginx for auth & share endpoints
- Health/readiness endpoints, JSON logs, basic metrics

### Out of Scope (MVP)
- Server-side sessions, refresh tokens, token blacklists
- Virus scanning, previews/thumbnails
- Full-text content search
- Multi-tenant organizations

---

## 4) Non-functional Requirements
- **Stateless API:** No server session storage
- **Availability:** ≥ 2 API replicas supported behind Nginx

### Performance Targets (local dev hardware)
- **P50** metadata requests ≤ 50 ms  
- **P95** metadata requests ≤ 200 ms (excluding object I/O)  
- List/search first page ≤ 300 ms @ ≤ 10k records

### Security
- Passwords: Argon2id or bcrypt (cost tuned)
- JWT HS256; TTL 15–30 min; strong secret via env
- Pre-signed URLs expire ≤ 10 min; validate content-type & size before presign
- Least privilege for MinIO bucket; no public listing

### Observability
- JSON structured logs, request IDs
- `/metrics` endpoint for Prometheus (basic counters/histograms)

### Accessibility
- WCAG 2.1 AA basics (labels, focus states, keyboard navigation)

### Internationalization
- English only (MVP), copy centralized

---

## 5) System Architecture

### Components
- **Nginx (RP/LB)** — Official `nginx:alpine`.  
  Host/path routing, WebSocket pass-through, round-robin to API replicas.
- **Frontend** — Vite + React SPA, compiled assets served by Nginx.
- **Backend API** — Go (Gin), stateless, JWT, presign to MinIO, metadata in Postgres; optional Redis caching.
- **Database** — `postgres:16-alpine` for users/files/shares/activities.
- **Object Storage** — `minio/minio` (S3-compatible).
- **Cache (optional)** — `redis:7-alpine` for hot metadata & rate-limit counters.
- **Admin (optional)** — `dpage/pgadmin4`, `redis-commander`.

---
