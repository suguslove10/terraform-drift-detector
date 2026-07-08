# Contributing to Terraform Drift Detector

Thank you for your interest in contributing! This guide will help you get started.

## Getting Started

1. **Fork** the repository
2. **Clone** your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/terraform-drift-detector.git
   cd terraform-drift-detector
   ```
3. **Install dependencies**:
   ```bash
   go mod tidy
   ```
4. **Build**:
   ```bash
   go build -o drift.exe .
   ```
5. **Run tests**:
   ```bash
   go test ./... -v
   ```

## How to Contribute

### Reporting Bugs
- Use GitHub Issues with a clear title and description
- Include steps to reproduce, expected vs actual behavior
- Include Go version (`go version`) and OS info

### Adding a New Cloud Provider

The provider system is designed to be extensible. To add a new provider:

1. Create a new file in `internal/provider/` (e.g., `gcp.go`)
2. Implement the `Provider` interface:
   ```go
   type Provider interface {
       Name() string
       FetchActual(ctx context.Context, resourceType, resourceID string, expectedAttrs map[string]any, expectedTags map[string]string) (*models.ResourceState, error)
       SupportedTypes() []string
   }
   ```
3. Register your provider in an `init()` function:
   ```go
   func init() {
       Register("gcp", &GCPProvider{})
   }
   ```
4. Add tests in `internal/provider/gcp_test.go`

### Submitting Changes

1. Create a feature branch: `git checkout -b feature/my-feature`
2. Make your changes with clear, descriptive commits
3. Ensure all tests pass: `go test ./... -v`
4. Run `go vet ./...` and fix any warnings
5. Push and open a Pull Request

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Write tests for new functionality
- Keep functions focused and documented
- Use meaningful variable and function names

## Project Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/` | CLI commands (Cobra) |
| `internal/models/` | Data models |
| `internal/parser/` | Terraform state parser |
| `internal/comparator/` | Diff engine |
| `internal/provider/` | Cloud provider drivers |
| `internal/scheduler/` | Background scheduler |
| `internal/store/` | JSON file storage |
| `web/` | API server & dashboard |
| `testdata/` | Sample state files |

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
