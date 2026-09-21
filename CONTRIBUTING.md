# Contributing to prunesh

Thanks for contributing. By participating you agree to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Where things go

| Change | Repository |
|---|---|
| Proxy, registry, install, hooks, agent integrations, built-in modules | [prunesh/prunesh](https://github.com/prunesh/prunesh) (this repo) |
| Community / niche plugins | [prunesh/marketplace](https://github.com/prunesh/marketplace) |

A plugin does not need to modify core. If an implementation needs to touch
`internal/` to work, that usually means the registry interface is incomplete —
open an issue here rather than forking core for one command.

## Development setup

Requirements:

- Go version from `go.mod` (1.22+)
- Git

```bash
git clone https://github.com/prunesh/prunesh.git
cd prunesh
go build -o prunesh ./cmd/prunesh/
go test ./...
go vet ./...
```

CI runs the same checks on every PR: build, test, vet, and version/manifest
consistency (see `.github/workflows/ci.yml`).

## Pull requests

1. Open an issue first for larger design changes (new modules, registry API,
   agent integration behavior).
2. Keep PRs focused. One concern per PR when practical.
3. Match existing style. Prefer small, surgical diffs.
4. Add or update tests next to the code you change.
5. Do not bump the version unless the PR is intentionally a release prep.
6. Ensure `go test ./...` and `go vet ./...` pass locally.

### Commit messages

Use clear, imperative subjects. Prefer Conventional Commits when it helps
(`fix:`, `feat:`, `docs:`). Explain *why* in the body when the change is not
obvious.

## Built-in modules

Built-in filters live in `plugins/` and register via `init()`. Wire them with a
blank import in `cmd/prunesh/main.go`.

Modules implement `registry.Module` (`Name`, `Rewrite`, `FilterOutput`,
`TokensBefore`, `TokensAfter`). Modules never import each other — shared logic
belongs in `internal/`.

Filtering is heuristic and deterministic: truncation, grouping, comment
stripping, line caps. No model calls, no semantic compression.

See the README section **Adding a built-in module** for a minimal skeleton.

## Marketplace plugins

External plugins speak `stdin/v1` over JSON stdin/stdout. Publish them through
the marketplace — do not open a PR here to ship a niche command.

- Authoring: [Writing a plugin](https://github.com/prunesh/prunesh#writing-a-plugin)
- Publish flow: [marketplace README](https://github.com/prunesh/marketplace#publishing-a-plugin)
- Catalog: https://prunesh.github.io/marketplace

## Version bumps (maintainers)

When changing the version, update every file that exposes it (see `AGENTS.md`):

- `cmd/prunesh/main.go`
- `.claude-plugin/plugin.json`
- `.claude-plugin/marketplace.json`
- `integrations/claude/.claude-plugin/plugin.json`
- `plugins/mcpscan/mcpscan.go`
- `README.md`

Order: bump → commit to `main` → tag. Never tag before the bump is on `main`.

## Security

Do not open public issues for vulnerabilities. Follow [SECURITY.md](SECURITY.md).

## Questions

- Product / usage: https://prunesh.github.io/prunesh
- Roadmap notes: [ROADMAP.md](ROADMAP.md)
- Bugs and features: [GitHub Issues](https://github.com/prunesh/prunesh/issues)
