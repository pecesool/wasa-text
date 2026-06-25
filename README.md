# WASA Text &middot; [![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/) [![Vue.js](https://img.shields.io/badge/Vue.js-3-4FC08D?logo=vuedotjs)](https://vuejs.org/) [![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://docker.com/) [![License](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

> **Enterprise-grade messaging architecture built from the ground up.**  
> A full-stack web messaging platform demonstrating clean architecture, containerized deployment, and production-ready engineering practices.

---

## 🚀 What This Project Delivers

**WASA Text** is a complete real-time messaging application engineered as a final project for the **Web and Software Architecture** course at Sapienza University of Rome. Unlike typical academic assignments, this codebase mirrors production standards: modular service layers, embedded web UI compilation, Docker containerization, and a fully documented REST API.

The application supports both **one-to-one direct messaging** and **group conversations**, with rich features including media attachments, message reactions, and conversation management — all built with zero reliance on external real-time services, demonstrating fundamental systems engineering competency.

---

## ✨ Core Features & Engineering Highlights

| Feature | Technical Implementation |
|---------|------------------------|
| **Direct & Group Messaging** | RESTful API with normalized SQLite schema, transactional message persistence |
| **Rich Media Support** | Image/GIF upload handling with binary storage optimization |
| **Message Reactions** | Associative reaction engine with Unicode emoji support |
| **Reply & Forward** | Contextual threading via message reference chains |
| **User Discovery** | Indexed username search with pattern matching |
| **Profile Management** | Avatar upload, username mutation with uniqueness constraints |
| **Embedded Web UI** | Vue.js SPA compiled and embedded directly into the Go binary via build tags |
| **Containerized Deployment** | Multi-stage Docker builds for both frontend and backend services |
| **Health Monitoring** | Dedicated healthcheck daemon for container orchestration readiness |

---

## 🏗️ Architecture & Design Decisions

### Backend (`service/`)
```
service/
├── api/           # HTTP handlers, middleware, request validation
├── globaltime/    # Injectable time wrapper (testability & determinism)
└── [domain]/      # Business logic isolated by bounded context
```

- **Clean Architecture**: Handlers → Services → Repositories layering with dependency injection
- **SQLite as Embedded Store**: Zero-config deployment, perfect for single-node and containerized environments
- **Structured Logging**: `logrus` for production-grade observability
- **UUID Generation**: Distributed-safe identifier generation without coordination

### Frontend (`webui/`)
- **Vue.js 3** with Composition API for reactive state management
- **Vue Router** client-side navigation with history mode
- **Axios** interceptors for centralized auth and error handling
- **Bootstrap Dashboard** template customized for messaging UX
- **Build-time Embedding**: Frontend assets compiled into the Go binary — single-executable deployment

### Operations (`cmd/`)
- **`cmd/webapi/`**: Production API server with graceful shutdown
- **`cmd/healthcheck/`**: Standalone health probe for Docker/Kubernetes liveness checks

---

## 🛠️ Technology Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Language** | Go 1.21+ | Performance, static binaries, excellent concurrency primitives |
| **Router** | `httprouter` | Zero-allocation, high-throughput HTTP routing |
| **Database** | SQLite | Serverless, embedded, ACID-compliant, perfect for portable deployments |
| **Frontend** | Vue.js 3 + Vite | Modern reactivity, fast HMR, optimized builds |
| **Styling** | Bootstrap 5 + Custom CSS | Responsive, accessible, rapid iteration |
| **Containerization** | Docker + Multi-stage builds | Minimal attack surface, reproducible environments |
| **Dependency Management** | Go Modules + Yarn Vendoring | Fully offline builds, supply-chain security |

---

## 🚦 Quick Start

### Prerequisites
- Go 1.21+
- Node.js 20+ (for UI development)
- Docker (optional, for containerized deployment)

### Backend Only
```bash
go run ./cmd/webapi/
# Server listens on :3000
```

### Full-Stack with Embedded UI
```bash
# Build frontend assets into Go binary
./open-node.sh
yarn run build-embed
exit

# Compile single binary with embedded UI
go build -tags webui ./cmd/webapi/
./webapi
```

### Docker Deployment
```bash
# Backend
docker build -f Dockerfile.backend -t wasa-text-backend:latest .

# Frontend (static served via nginx)
docker build -f Dockerfile.frontend -t wasa-text-frontend:latest .

# Run
docker run -p 3000:3000 wasa-text-backend:latest
```

---

## 📐 API Specification

Fully documented via **OpenAPI 3.0** — see [`doc/api.yaml`](doc/api.yaml) for complete endpoint definitions, request/response schemas, and authentication flows.

---

## 🎯 What This Project Demonstrates

> **"This isn't a tutorial clone. It's original architecture with production intent."**

- **Systems Thinking**: Designed service boundaries before writing code; database schema reflects domain model, not ORM defaults
- **DevOps Mindset**: Docker multi-stage builds, health checks, and container-ready configuration
- **Frontend Engineering**: SPA architecture with state management, optimistic UI updates, and responsive design
- **Go Idioms**: Context propagation, error wrapping, interface-driven design, and build tag conditional compilation
- **Security Awareness**: Input validation, SQL parameterization, XSS prevention via Vue's auto-escaping

---

## 📜 License

BSD 3-Clause — see [LICENSE](LICENSE) for details.

---

**Maintainer**: [Zhassulan Baimyshev](https://www.linkedin.com/in/zhassulan-baimyshev/)  
*Built for the Web and Software Architecture course, Sapienza University of Rome*
