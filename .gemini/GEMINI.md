## Gemini Added Memories
- The project uses HTMX for dynamic UI updates and Go templates for server-side rendering. Generated images are stored locally in 'tmp/xai_generations/' for review before being uploaded to GCS.
- Always run `golangci-lint run` (or the project's equivalent lint command) and fix any reported issues before finalizing changes.
- In Go files, ensure all error return values are checked. For `defer x.Close()` calls where the error can be safely ignored, wrap them in an anonymous function and explicitly ignore the error (e.g., `defer func() { _ = x.Close() }()`) to satisfy `errcheck`.
- **CRITICAL:** NEVER bypass git hooks. Do not use `--no-verify` or any other method to skip pre-commit checks, even if tools like `govulncheck` are crashing or failing. If a hook fails, you must investigate and fix the root cause or ask the user for guidance.
- **Workflow:** ALWAYS check if a person already exists in the database using `search-asian-americans` before attempting to add a new one.
- **Maintenance:** To update the Go version, you must update:
  1. `go.mod` (e.g., `go mod edit -go 1.26.1`)
  2. `Dockerfile` (`ARG GO_VERSION=...`)
  3. Pre-commit hooks and CI usually follow `go.mod`, but verify if any tools (like `golangci-lint` or `govulncheck`) need a version bump to support the new Go release.
- **Binary Hygiene:** NEVER commit compiled binaries (e.g., Go executables) to the repository. Always ensure they are listed in `.gitignore`.
