# CryptoFunk Versioning Strategy

## Current Version

**Version:** 1.0.0

This is the canonical version managed in `internal/config/version.go`.

## Semantic Versioning

CryptoFunk follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR version** (1.x.x): Incompatible API changes
- **MINOR version** (x.1.x): New functionality in a backwards-compatible manner
- **PATCH version** (x.x.1): Backwards-compatible bug fixes

## Version Management

### Single Source of Truth

The canonical version is defined in `internal/config/version.go`:

```go
package config

// Version is the canonical version of CryptoFunk
const Version = "1.0.0"

func GetVersion() string {
    return Version
}
```

### Usage in Code

All Go code should import and use the centralized version:

```go
import "github.com/ajitpratap0/cryptofunk/internal/config"

// Use config.Version or config.GetVersion()
version := config.Version
```

### Version References

The version is used in:

1. **MCP Servers**: All MCP servers report the canonical version in their initialize response
   - `cmd/mcp-servers/market-data/main.go`
   - `cmd/mcp-servers/technical-indicators/main.go`
   - `cmd/mcp-servers/risk-analyzer/main.go`
   - `cmd/mcp-servers/order-executor/main.go`

2. **Agents**: All agents read version from config
   - Agent configurations inherit version from config defaults

3. **API Server**: Reports version in health endpoints
   - `GET /health` returns system version
   - `GET /api/v1/version` returns version details

4. **CLI Tools**: Taskfile version command
   - `task version` displays current version

5. **Docker Images**: Tagged with semantic version
   - `cryptofunk/orchestrator:1.0.0`
   - `cryptofunk/api:1.0.0`
   - Also tagged with `:latest` for convenience

6. **Documentation**: Version headers in major docs
   - README.md
   - ARCHITECTURE.md
   - API.md
   - DEPLOYMENT.md

## Updating the Version

To update the version across the entire system:

1. **Update the canonical version** in `internal/config/version.go`:
   ```go
   const Version = "1.1.0"
   ```

2. **Update documentation headers** (if major/minor version change):
   - README.md
   - ARCHITECTURE.md
   - TASKS.md
   - API.md
   - DEPLOYMENT.md
   - MCP_GUIDE.md

3. **Rebuild all binaries**:
   ```bash
   task build
   ```

4. **Rebuild Docker images** (if deploying):
   ```bash
   docker-compose -f deployments/docker-compose.yml build
   ```

5. **Update Kubernetes manifests** (if using K8s):
   ```bash
   # Update image tags in deployments/k8s/base/*.yaml
   # Or use Kustomize to override image tags
   ```

6. **Commit the change**:
   ```bash
   git add internal/config/version.go docs/
   git commit -m "chore: Bump version to 1.1.0"
   git tag v1.1.0
   git push origin main --tags
   ```

## CI/CD Integration

The CI/CD pipeline automatically:

1. **Extracts version** from `internal/config/version.go`
2. **Tags Docker images** with the version
3. **Creates GitHub releases** on version tags (v*)
4. **Updates deployment manifests** with new image tags

## Version History

### 1.0.0 (Current)
- Initial production release
- All 10 phases complete
- LLM-powered multi-agent trading system
- Full MCP integration
- Docker and Kubernetes deployment support
- CI/CD pipeline with automated testing
- Production-ready monitoring and observability

### 0.1.0 (Development)
- Initial development version
- Used during Phase 1-9 implementation
- Not recommended for production use

## References

- **Semantic Versioning**: https://semver.org/
- **Go Module Versioning**: https://go.dev/doc/modules/version-numbers
- **Docker Image Tagging**: https://docs.docker.com/engine/reference/commandline/tag/
- **Git Tagging**: https://git-scm.com/book/en/v2/Git-Basics-Tagging
