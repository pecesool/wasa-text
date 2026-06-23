# Wasa Text

Go backend + web UI (text processing project)

Maintainer: Zhassulan Baimyshev — https://www.linkedin.com/in/zhassulan-baimyshev/

Summary

A Go-based project providing text processing services with a web UI. The repo contains a `service` backend, `webui`, `cmd` directory, and Dockerfiles for frontend/backend.

Tech

- Language: Go
- Build: `go.mod` present
- Docker: `Dockerfile.backend`, `Dockerfile.frontend`

Quick start (backend)

1. Build backend:

```bash
cd service
go build ./...
```

2. Run the service (example):

```bash
./service
```

Docker (example)

```bash
docker build -f Dockerfile.backend -t wasan-backend:latest .
# build frontend similarly with Dockerfile.frontend
```

Notes

- I can add a docker-compose manifest and example environment variables if you want a one-command developer experience.
