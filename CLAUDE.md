# Portainer Terraform Provider

## Project Overview
Terraform provider for managing Portainer resources. Written in Go, uses the HashiCorp Terraform Plugin SDK v2 and the Portainer client API Go SDK.

## Build & Test

```bash
make build              # Compile the provider binary
make install-plugin     # Install to local Terraform plugin directory
make fmt-check          # Check Terraform formatting
make fmt                # Format Terraform files
make go-fmt-check       # Check Go formatting
make go-fmt             # Format Go files
make validate           # Validate all Terraform configs
```

## Project Structure

- `internal/` - All provider Go source code (resources, data sources, provider config)
  - `resource_*.go` - Resource implementations (CRUD)
  - `data_source_*.go` - Data source implementations
  - `provider.go` - Provider registration, schema, and resource/data source maps
- `docs/resources/` - Resource documentation (Markdown)
- `docs/data-sources/` - Data source documentation
- `examples/` - Example Terraform configurations per resource
- `e2e-tests/` - End-to-end test Terraform configs (run in CI)
- `.github/workflows/` - CI/CD (PR e2e tests, daily e2e tests, release, linting)

## Key Conventions

### Resource Implementation Pattern
Each resource follows this structure:
1. Schema definition with `schema.Resource` struct
2. CRUD functions: `resourcePortainer<Name>Create/Read/Update/Delete`
3. API calls use either the generated SDK client (`client.Client.Endpoints.*`) or direct HTTP requests (`http.NewRequest`)
4. Authentication via `X-API-Key` header or `Authorization: Bearer` JWT token

### API Client
Two patterns are used for API interaction:
- **Generated SDK** (`client.Client.*`): Used for environment, endpoint group resources
- **Direct HTTP**: Used for most other resources - manual JSON marshaling, direct `http.NewRequest`

### Naming
- Terraform resource names: `portainer_<resource>` (snake_case)
- Go functions: `resourcePortainer<Resource>Create` (PascalCase)
- API JSON fields: camelCase (e.g., `edgeGroupIDs`, `stackName`)

### Environment Types
- 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent
- Portainer converts Edge Agent type 4 to type 7 after agent connects (handled via DiffSuppressFunc)
- Edge Agent (type 4/7): Tags must be applied via Update after creation (not in Create form params)

## E2E/PR Tests

Tests are organized in `e2e-tests/<resource>/` directories. Each has `main.tf`, `variables.tf`, and a resource `.tf` file.

CI runs tests in phases:
1. Setup (Portainer, k3d, kubectl)
2. Environment creation
3. Full-cycle tests (apply + destroy)
4. Apply-only tests
5. Swarm-specific tests
6. Edge agent tests
7. SSL test

Default test credentials:
- URL: `https://localhost:9443`
- API key: `ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8=`
