## Gemini Added Memories
- The project uses HTMX for dynamic UI updates and Go templates for server-side rendering. Generated images are stored locally in 'tmp/xai_generations/' for review before being uploaded to GCS.
- Always run `golangci-lint run` (or the project's equivalent lint command) and fix any reported issues before finalizing changes.
- In Go files, ensure all error return values are checked. For `defer x.Close()` calls where the error can be safely ignored, wrap them in an anonymous function and explicitly ignore the error (e.g., `defer func() { _ = x.Close() }()`) to satisfy `errcheck`.
- **CRITICAL:** NEVER bypass git hooks. Do not use `--no-verify` or any other method to skip pre-commit checks, even if tools like `govulncheck` are crashing or failing. If a hook fails, you must investigate and fix the root cause or ask the user for guidance.
