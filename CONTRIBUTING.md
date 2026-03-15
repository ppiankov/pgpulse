# Contributing to pgpulse

## Getting started

```bash
git clone https://github.com/ppiankov/pgpulse.git
cd pgpulse
make build
make test
```

## Development workflow

1. Create a branch from `main`
2. Make your changes
3. Run `make verify` (builds, tests with race detection, lints)
4. Commit with [conventional commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`
5. Open a pull request against `main`

## Code style

- Go files: `snake_case.go`
- Packages: short single-word names
- Run `make fmt` before committing
- Run `make lint` to check for issues

## Testing

- Tests are mandatory for new code
- Use `-race` flag (already in `make test`)
- Test files go alongside source files
- Use the `Querier` interface for mocking database calls

## Project structure

```
cmd/pgpulse/        CLI entry point
internal/
  cli/              Cobra commands
  config/           Environment configuration
  collector/        Poll loop and metric collectors
  metrics/          Prometheus metric definitions
```

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
