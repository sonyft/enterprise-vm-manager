# Enterprise VM Manager

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue?style=for-the-badge&logo=docker)](Dockerfile)
[![API](https://img.shields.io/badge/API-REST-green?style=for-the-badge)](http://localhost:8080/swagger/index.html)

**Enterprise VM Manager** is a production-ready REST API for managing virtual machines, built with **modern Go practices** and **cloud-native architecture**. This project demonstrates **enterprise-grade development patterns** suitable for **Infrastructure-as-a-Service (IaaS)** platforms like STACKIT.

## 🌟 Key Features

### **🖥️ VM Lifecycle Management**
- **CRUD Operations**: Create, Read, Update, Delete virtual machines
- **State Management**: Start, Stop, Restart, Suspend, Resume operations
- **Resource Allocation**: CPU, RAM, Disk configuration with validation
- **Network Configuration**: NAT, Bridge, Host networking modes
- **Node Assignment**: Automatic distribution across compute nodes

### **📊 Monitoring & Analytics**
- **Real-time Statistics**: CPU, RAM, Disk, Network usage tracking
- **Resource Summary**: System-wide utilization overview
- **Performance Metrics**: Prometheus-compatible metrics endpoint
- **Health Checks**: Kubernetes-ready liveness and readiness probes

### **🔒 Security & Authentication**
- **API Key Authentication**: Configurable API key validation
- **JWT Support**: Token-based authentication (ready for integration)
- **Rate Limiting**: Configurable request rate limiting
- **CORS Support**: Cross-origin request handling
- **Security Headers**: OWASP-recommended security headers

### **🏗️ Enterprise Architecture**
- **Clean Architecture**: Repository-Service-Handler pattern
- **Dependency Injection**: Proper component separation
- **Error Handling**: Comprehensive error types and responses
- **Structured Logging**: JSON-formatted logs with request tracing
- **Configuration Management**: Environment-based configuration

### **🚀 Production Ready**
- **Docker Support**: Multi-stage builds for optimal images
- **Kubernetes Ready**: Health checks and graceful shutdown
- **Database Migrations**: Version-controlled schema changes
- **Comprehensive Testing**: Unit, integration, and API tests
- **OpenAPI Documentation**: Swagger-generated API documentation

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   REST API      │    │   Business       │    │   Data          │
│   (Handlers)    │◄───│   Logic          │◄───│   Layer         │
│                 │    │   (Services)     │    │   (Repository)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Middleware    │    │   Models &       │    │   PostgreSQL    │
│   (Auth, CORS,  │    │   Validation     │    │   Database      │
│   Rate Limit)   │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### **Directory Structure**

```
enterprise-vm-manager/
├── cmd/                      # Application entry points
│   ├── server/              # Main API server
│   └── cli/                 # Command-line tool (vmctl)
├── internal/                # Private application code
│   ├── api/                 # HTTP layer
│   │   ├── handlers/       # HTTP request handlers
│   │   ├── middleware/     # HTTP middleware components
│   │   └── routes/         # Route definitions
│   ├── config/             # Configuration management
│   ├── database/           # Database layer
│   │   └── migrations/    # SQL migration files
│   ├── models/             # Domain models and DTOs
│   ├── repositories/       # Data access layer
│   ├── services/           # Business logic layer
│   └── utils/              # Internal utilities
├── pkg/                     # Public library code
│   ├── errors/             # Error types and handling
│   ├── logger/             # Structured logging
│   ├── validator/          # Input validation
│   └── httputil/           # HTTP utilities
├── api/                     # API definitions
│   └── openapi/           # OpenAPI/Swagger specifications
├── configs/                # Configuration files
├── deployments/            # Deployment configurations
│   ├── docker/            # Docker-specific files
│   ├── kubernetes/        # Kubernetes manifests
│   └── helm/              # Helm charts
├── test/                   # Test files
│   ├── unit/              # Unit tests
│   ├── integration/       # Integration tests
│   └── fixtures/          # Test data
├── docs/                   # Documentation
├── scripts/                # Build and deployment scripts
└── build/                  # Compiled binaries
```

## 🚀 Quick Start

### **Prerequisites**

- **Go 1.21+** - [Download](https://golang.org/dl/)
- **Docker & Docker Compose** - [Install](https://docs.docker.com/get-docker/)
- **PostgreSQL 15+** (if running locally) - [Install](https://www.postgresql.org/download/)
- **Make** (optional, for convenience commands)

### **Option 1: Docker Compose (Recommended)**

```bash
# Clone the repository
git clone https://github.com/stackit/enterprise-vm-manager.git
cd enterprise-vm-manager

# Start all services (API + PostgreSQL + pgAdmin)
docker-compose up -d

# Check service status
docker-compose ps

# View API logs
docker-compose logs -f api
```

**🎉 API will be available at:**
- **API Endpoints**: http://localhost:8080
- **Swagger Documentation**: http://localhost:8080/swagger/index.html
- **Health Check**: http://localhost:8080/health
- **pgAdmin**: http://localhost:5050 (admin@vmmanager.local / admin123)

### **Option 2: Local Development**

```bash
# Clone the repository
git clone https://github.com/stackit/enterprise-vm-manager.git
cd enterprise-vm-manager

# Install dependencies
make deps

# Start PostgreSQL (Docker)
docker-compose up -d postgres

# Copy environment variables
cp .env.example .env

# Run database migrations
make migrate-up

# Start the API server
make run
# OR with hot reload
make run-dev
```

### 1) Инсталирай migrate CLI (за да работят Makefile миграциите)
Препоръчително с Homebrew: ``` brew install golang-migrate ```

### 2) Стартирай/инсталирай локален Postgres
Ако Postgres не е стартиран:
  ``` bash
  brew services start postgresql@16
  # или:
  pg_ctl -D /usr/local/var/postgresql@16 start
  ```

### 3) Създай роля и база + разширение за UUID
Приложението използва default gen_random_uuid(); нужна е pgcrypto.
``` bash
  # Влез в psql
  psql postgres

  -- В psql:
  CREATE ROLE vmmanager WITH LOGIN PASSWORD 'password123';
  ALTER ROLE vmmanager CREATEDB;  -- по желание
  CREATE DATABASE vmmanager OWNER vmmanager;

  \c vmmanager
  CREATE EXTENSION IF NOT EXISTS "pgcrypto";
  \q
```

### 4) Задай DATABASE_URL и пусни миграциите от Makefile
``` bash
  export DATABASE_URL="postgres://vmmanager:password123@localhost:5432/vmmanager?sslmode=disable"

  make migrate-up
  # връщане назад (ако трябва):
  # make migrate-down
```


### **Option 3: Build from Source**

```bash
# Build binaries
make build

# Run server
./build/enterprise-vm-manager-server

# Use CLI tool
./build/vmctl --help
```

## 📡 API Usage Examples

### **Create a Virtual Machine**

```bash
curl -X POST http://localhost:8080/api/v1/vms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-server-01",
    "description": "Production web server",
    "cpu_cores": 4,
    "ram_mb": 8192,
    "disk_gb": 100,
    "image_name": "ubuntu:22.04",
    "network_type": "nat",
    "created_by": "user123"
  }'
```

**Response:**
```json
{
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "web-server-01",
    "status": "pending",
    "spec": {
      "cpu_cores": 4,
      "ram_mb": 8192,
      "disk_gb": 100
    },
    "created_at": "2025-10-15T22:00:00Z"
  },
  "message": "VM created successfully",
  "request_id": "req-abc123"
}
```

### **List Virtual Machines**

```bash
# List all VMs
curl http://localhost:8080/api/v1/vms

# Filter by status
curl "http://localhost:8080/api/v1/vms?status=running&limit=10"

# Search VMs
curl "http://localhost:8080/api/v1/vms?search=web-server"
```

### **Start a Virtual Machine**

```bash
curl -X POST http://localhost:8080/api/v1/vms/123e4567-e89b-12d3-a456-426614174000/start \
  -H "Content-Type: application/json"
```

### **Get Resource Summary**

```bash
curl http://localhost:8080/api/v1/stats/summary
```

**Response:**
```json
{
  "data": {
    "vms": {
      "total": 10,
      "running": 7,
      "stopped": 3
    },
    "resources": {
      "cpu": {
        "total": 40,
        "used": 28,
        "usage_percent": 70.0
      },
      "ram": {
        "total_mb": 81920,
        "used_mb": 57344,
        "usage_percent": 70.0
      }
    }
  }
}
```

## 🛠️ CLI Tool (vmctl)

The project includes a powerful CLI tool for managing VMs from the command line:

```bash
# List VMs
./vmctl vm list

# Get VM details
./vmctl vm get 123e4567-e89b-12d3-a456-426614174000

# Create a new VM
./vmctl vm create my-new-vm --cpu 2 --ram 4096 --disk 50

# Start a VM
./vmctl vm start 123e4567-e89b-12d3-a456-426614174000

# Get system statistics
./vmctl stats

# Generate shell completions
./vmctl completion bash > /etc/bash_completion.d/vmctl
```

## 🧪 Testing

### **Run All Tests**

```bash
# Unit tests
make test-unit

# Integration tests
make test-integration

# All tests with coverage
make test-coverage

# View coverage report
open coverage.html
```

### **Manual API Testing**

```bash
# Test API endpoints
make test-api

# Load testing
make benchmark

# Health check
curl http://localhost:8080/health
```

## 🔧 Configuration

### **Environment Variables**

Key environment variables for configuration:

```bash
# Database
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=disable

# Server
VM_MANAGER_SERVER_HOST=0.0.0.0
VM_MANAGER_SERVER_PORT=8080
VM_MANAGER_SERVER_MODE=release  # debug, release, test

# Authentication
VM_MANAGER_AUTH_ENABLED=true
JWT_SECRET=your-secret-key
API_KEY_PRIMARY=your-api-key

# Logging
VM_MANAGER_LOGGING_LEVEL=info
VM_MANAGER_LOGGING_FORMAT=json

# Resource Limits
VM_MANAGER_LIMITS_MAX_CPU_CORES=64
VM_MANAGER_LIMITS_MAX_RAM_MB=262144
VM_MANAGER_LIMITS_MAX_DISK_GB=10240
```

### **Configuration Files**

- `configs/config.yaml` - Main configuration
- `configs/config.dev.yaml` - Development overrides
- `configs/config.prod.yaml` - Production settings

## 🐳 Docker & Container Support

### **Multi-stage Production Build**

```dockerfile
# Optimized for production
FROM golang:1.21-alpine AS builder
# ... build process
FROM alpine:3.19
# ... minimal runtime
```

### **Docker Compose Profiles**

```bash
# Basic stack (API + Database)
docker-compose up -d

# With development tools
docker-compose --profile dev-tools up -d

# With monitoring (Prometheus + Grafana)
docker-compose --profile monitoring up -d

# With caching (Redis)
docker-compose --profile with-cache up -d
```

### **Kubernetes Deployment**

```bash
# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/

# Or use Helm
helm install vm-manager deployments/helm/vm-manager/
```

## 🚀 Production Deployment

### **System Requirements**

**Minimum:**
- 2 CPU cores, 4GB RAM
- PostgreSQL 15+
- 10GB disk space

**Recommended:**
- 4+ CPU cores, 8GB+ RAM
- PostgreSQL with read replicas
- 50GB+ disk space
- Load balancer (nginx, HAProxy)
- Monitoring stack (Prometheus, Grafana)

### **Production Checklist**

- [ ] **Security**: Change default passwords and API keys
- [ ] **TLS**: Enable HTTPS with valid certificates
- [ ] **Database**: Use managed PostgreSQL with backups
- [ ] **Monitoring**: Set up Prometheus + Grafana
- [ ] **Logging**: Configure centralized log aggregation
- [ ] **Scaling**: Configure horizontal pod autoscaling
- [ ] **Backups**: Implement database backup strategy
- [ ] **Secrets**: Use proper secret management (Vault, K8s secrets)

### **Environment-specific Deployment**

```bash
# Development
make docker-run

# Staging
docker-compose -f docker-compose.yml -f docker-compose.staging.yml up -d

# Production
make k8s-deploy
# OR
helm install vm-manager ./deployments/helm/vm-manager \
  --set image.tag=v1.0.0 \
  --set database.host=prod-postgres.example.com
```

## 📊 Monitoring & Observability

### **Built-in Metrics**

- **HTTP Metrics**: Request duration, status codes, throughput
- **Business Metrics**: VM counts by status, resource utilization
- **System Metrics**: Database connections, memory usage
- **Custom Metrics**: API-specific performance indicators

### **Health Checks**

```bash
# Application health
curl http://localhost:8080/health

# Readiness probe (Kubernetes)
curl http://localhost:8080/ready

# Liveness probe (Kubernetes)
curl http://localhost:8080/live

# Metrics (Prometheus)
curl http://localhost:8080/metrics
```

### **Monitoring Stack**

The project includes a complete monitoring setup:

```bash
# Start monitoring stack
docker-compose --profile monitoring up -d

# Access dashboards
open http://localhost:9090  # Prometheus
open http://localhost:3000  # Grafana (admin/admin123)
```

## 🔐 Security Considerations

### **Authentication & Authorization**

```bash
# Enable authentication
export VM_MANAGER_AUTH_ENABLED=true
export JWT_SECRET=your-super-secret-key

# Use API keys
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/vms
```

### **Rate Limiting**

```yaml
server:
  rate_limit:
    enabled: true
    rps: 100.0        # Requests per second
    burst: 200        # Burst capacity
    cleanup: "1m"     # Cleanup interval
```

### **Security Headers**

The API automatically adds security headers:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security` (when HTTPS)

## 🛠️ Development

### **Prerequisites for Development**

```bash
# Install development tools
make install-tools

# This installs:
# - air (hot reload)
# - golangci-lint (linting)
# - gosec (security scanning)
# - swag (OpenAPI generation)
# - migrate (database migrations)
```

### **Development Workflow**

```bash
# Set up development environment
make env-dev

# Start with hot reload
make run-dev

# Run linting
make lint

# Run security scan
make security

# Format code
make fmt

# Run all checks
make check
```

### **Database Migrations**

```bash
# Create new migration
make migrate-create NAME=add_vm_labels

# Run migrations
make migrate-up

# Rollback migrations
make migrate-down

# Force specific version
make migrate-force VERSION=1
```

### **API Documentation**

```bash
# Generate OpenAPI docs
make swagger

# View documentation
open http://localhost:8080/swagger/index.html
```

## 📈 Performance & Scaling

### **Performance Benchmarks**

```bash
# Run benchmarks
make benchmark

# Typical results (local machine):
# - Create VM: ~50ms
# - List VMs (100): ~20ms
# - Get VM: ~5ms
# - Throughput: ~2000 req/s
```

### **Scaling Considerations**

**Horizontal Scaling:**
- Stateless API design allows multiple replicas
- Database connection pooling
- Load balancer with session affinity

**Database Scaling:**
- Read replicas for query performance
- Connection pooling (pgbouncer)
- Partitioning for large VM counts

**Caching:**
- Redis for session storage
- Application-level caching for resource summaries
- CDN for static assets

## 🤝 Contributing

We welcome contributions! Please follow these guidelines:

### **Development Setup**

```bash
# Fork the repository on GitHub
git clone https://github.com/your-username/enterprise-vm-manager.git

# Create feature branch
git checkout -b feature/my-new-feature

# Make changes and test
make check test

# Commit with conventional commits
git commit -m "feat: add VM snapshots support"

# Push and create pull request
git push origin feature/my-new-feature
```

### **Code Quality Standards**

- **Go Style**: Follow [Effective Go](https://golang.org/doc/effective_go.html)
- **Testing**: Maintain >80% test coverage
- **Documentation**: Update README and API docs
- **Commit Messages**: Use [Conventional Commits](https://conventionalcommits.org/)

### **Pull Request Process**

1. Ensure tests pass: `make check test`
2. Update documentation if needed
3. Add entry to CHANGELOG.md
4. Request review from maintainers

## 📋 Roadmap

### **Version 1.1 (Planned)**
- [ ] VM snapshots and cloning
- [ ] Network security groups
- [ ] VM migration between nodes
- [ ] WebSocket real-time updates
- [ ] GraphQL API support

### **Version 1.2 (Future)**
- [ ] Multi-tenant support
- [ ] VM scheduling policies
- [ ] Resource quotas and billing
- [ ] Integration with cloud providers
- [ ] VM template management

## ❓ FAQ

**Q: Can this manage real VMs?**
A: This is a simulation/demo API. For real VM management, integrate with hypervisors like KVM, VMware, or cloud APIs.

**Q: Is this production-ready?**
A: Yes! The code follows production best practices, but you should customize it for your specific infrastructure.

**Q: How do I add authentication?**
A: Set `VM_MANAGER_AUTH_ENABLED=true` and configure API keys or JWT tokens in the config.

**Q: Can I use a different database?**
A: The code uses GORM, so you can easily switch to MySQL, SQLite, or other GORM-supported databases.

**Q: How do I deploy to Kubernetes?**
A: Use the provided Kubernetes manifests in `deployments/kubernetes/` or the Helm chart.

## 📞 Support

- **Documentation**: [GitHub Wiki](https://github.com/stackit/enterprise-vm-manager/wiki)
- **Issues**: [GitHub Issues](https://github.com/stackit/enterprise-vm-manager/issues)
- **Discussions**: [GitHub Discussions](https://github.com/stackit/enterprise-vm-manager/discussions)
- **Security**: Report security issues to security@stackit.example.com

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **STACKIT** - For inspiration on cloud infrastructure patterns
- **Gin Web Framework** - For excellent HTTP handling
- **GORM** - For elegant database operations
- **Go Community** - For amazing ecosystem and tools

---

**Built with ❤️ for STACKIT and the Go community**

> This project demonstrates enterprise-grade Go development practices suitable for cloud infrastructure platforms. Perfect for learning modern API development, clean architecture, and production deployment patterns.
