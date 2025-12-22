# chartup

CLI tool to check Helm charts and Docker images for version updates.

## Features

- Scans directories for `Chart.yaml` and `values.yaml` files
- Checks Docker registries for newer image tags (Docker Hub, Quay.io, ghcr.io, gcr.io, registry.k8s.io)
- Checks ArtifactHub for Helm chart updates (Bitnami, Trino)
- Clickable file links in terminal (opens in your editor)
- JSON cache to avoid repeated API calls
- Stops gracefully on rate limits

## Installation

```bash
go install github.com/nogo/chartup@latest
```

Or build from source:

```bash
git clone https://github.com/nogo/chartup.git
cd chartup
go build -o chartup .
```

## Usage

```bash
# Scan current directory
chartup .

# Scan specific path
chartup /path/to/helm/charts

# Ignore cache
chartup --no-cache .

# Set cache TTL
chartup --cache-ttl 24h .

# Specify editor for links (auto-detects from $EDITOR)
chartup --editor vscode .
```

## Options

| Flag | Description |
|------|-------------|
| `--no-cache` | Ignore cached results |
| `--cache-ttl` | Cache validity duration (default: 1h) |
| `--editor` | Editor for file links: `vscode`, `cursor`, `idea`, `sublime`, `zed`, `none` |
| `--version` | Show version |

## Supported Editors

The `--editor` flag configures clickable links in terminal output. If not set, auto-detects from `$EDITOR` or `$VISUAL` environment variables.

| Editor | Flag value |
|--------|------------|
| VS Code | `vscode` |
| Cursor | `cursor` |
| JetBrains IDEs | `idea` |
| Sublime Text | `sublime` |
| Zed | `zed` |
| Disable links | `none` |

## Supported Registries

| Registry | Notes |
|----------|-------|
| Docker Hub | Official + community images |
| Quay.io | Red Hat, MinIO, etc. |
| ghcr.io | GitHub Container Registry |
| gcr.io | Google Container Registry |
| registry.k8s.io | Kubernetes images |

## Example Output

```
DOCKER IMAGES
════════════════════════════════════════════════════════════════════════════════
📄 1_setup/6_trino/values.yaml
┌───────────────┬─────────┬────────┬──────────┬──────┐
│ REPOSITORY    │ CURRENT │ LATEST │ STATUS   │ LINE │
├───────────────┼─────────┼────────┼──────────┼──────┤
│ trinodb/trino │ 410     │ 479    │ ⚠ UPDATE │    7 │
│ busybox       │ 1.28    │ 1.37.0 │ ⚠ UPDATE │  163 │
└───────────────┴─────────┴────────┴──────────┴──────┘

HELM CHARTS
════════════════════════════════════════════════════════════════════════════════
📄 1_setup/6_trino/Chart.yaml
┌───────┬──────────┬─────────┬────────┬──────────┐
│ CHART │ UPSTREAM │ CURRENT │ LATEST │ STATUS   │
├───────┼──────────┼─────────┼────────┼──────────┤
│ trino │ trinodb  │ 0.8.0   │ 1.41.0 │ ⚠ UPDATE │
└───────┴──────────┴─────────┴────────┴──────────┘
```

## License

MIT
