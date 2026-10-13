# Order & Inventory Microservice Platform (Starter Implementation)

A runnable local demo of a retail backend split into four Go services:
- API Gateway (HTTP routing)
- Order Service
- Inventory Service
- Catalog Service

MongoDB is used for persistence. The demo includes Docker Compose, basic REST endpoints,
a compound-index initialization script, and GitHub Actions tests.

> This is a portfolio-ready starter implementation, not evidence of the production metrics in
> the sample resume bullets. Benchmark and validate any performance/coverage claims yourself.

## Run
Requirements: Docker + Docker Compose.

```bash
docker compose up --build
```

Gateway: `http://localhost:8080`
Health: `GET /health`
Catalog: `GET /api/products`
Orders: `POST /api/orders` with JSON `{"productId":"sku-1","quantity":2,"userId":"demo-user"}`
Inventory: `GET /api/inventory/sku-1`

## Notes
- The gateway routes to the internal services by Docker service name.
- MongoDB collections and indexes are initialized by the services.
- For production use, add authentication, request IDs, retries/timeouts, migrations,
  structured logging, secrets management, idempotency, and integration tests.
