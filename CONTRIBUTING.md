# Contributing

Thanks for contributing to `frisco`.

## Local setup

Requirements:

- Go version from `go.mod`

Build and run:

```bash
make build
./bin/frisco --help
```

## Quality checks

Before opening a PR, run:

```bash
go test ./...
go vet ./...
go build ./cmd/frisco
```

## Security and secrets

- Never commit local session files or credentials.
- Do not include real tokens/cookies in examples, logs, or test fixtures.
- Security reports should follow `SECURITY.md`.

## Pull requests

- Keep PRs focused and small when possible.
- Update `README.md` if CLI flags/behavior change.
- Add or update tests for behavior changes.
