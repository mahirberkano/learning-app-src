# learning-app-src

A minimal Go microservice built for learning Kubernetes scaling concepts (HPA, Karpenter).

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Returns pod info (hostname, version, redis status, go version) |
| `/healthz` | GET | Health check — verifies Redis connectivity |
| `/load?duration=5s` | GET | Burns CPU for the specified duration (max 60s) — used to trigger HPA |
| `/metrics` | GET | Prometheus metrics (request counts, durations) |
| `/ui/` | GET | Web UI — visualizes load balancing and provides a load generator button |

## Architecture

```
┌──────────────┐       ┌───────────┐
│ learning-app │──────▶│   Redis   │
│   (Go HTTP)  │       │  (cache)  │
└──────────────┘       └───────────┘
```

- **Language:** Go 1.22
- **Image size:** ~12MB (multi-stage build, scratch base)
- **Config:** `REDIS_ADDR` env var (default: `localhost:6379`)
- **Metrics:** Prometheus client (`http_requests_total`, `http_request_duration_seconds`)

## Local Development

```bash
# Build the Docker image
docker build -t learning-app:dev .

# Run locally (needs Redis)
docker run -d --name redis -p 6379:6379 redis:7-alpine
docker run --rm -p 8080:8080 -e REDIS_ADDR=host.docker.internal:6379 learning-app:dev

# Test
curl http://localhost:8080/
curl http://localhost:8080/healthz
curl http://localhost:8080/ui/
```

## CI/CD Pipeline

On every push to `main`, GitHub Actions:

1. Builds the Docker image
2. Tags with the git SHA (e.g., `mahirberkan/learning-app:a3f9b2c`)
3. Pushes to Docker Hub
4. Updates the image tag in the [gitops repo](https://github.com/mahirberkano/learning-app-gitops)
5. ArgoCD detects the change and deploys automatically

```
Push code → GitHub Actions → Docker Hub → gitops repo updated → ArgoCD syncs → new pods
```

## Required GitHub Secrets

| Secret | Purpose |
|--------|---------|
| `DOCKERHUB_USERNAME` | Docker Hub login |
| `DOCKERHUB_TOKEN` | Docker Hub access token (read/write) |
| `GIT_TOKEN` | GitHub PAT with repo scope (to push to gitops repo) |

## Related

- [learning-app-gitops](https://github.com/mahirberkano/learning-app-gitops) — Kubernetes manifests + ArgoCD config
