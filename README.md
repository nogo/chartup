# chartup

CLI tool to check Helm charts and Docker images for version updates.

## Features

- Scans directories for `Chart.yaml`, `values.yaml`, and Dockerfiles
- Extracts images from Dockerfiles (`FROM` instructions with ARG variable resolution)
- Checks Docker registries for newer image tags (Docker Hub, Quay.io, ghcr.io, gcr.io, registry.k8s.io)
- Checks ArtifactHub for Helm chart updates (Bitnami, Trino)
- Filters out pre-release versions (-dev, -alpha, -beta, -rc, etc.)
- Clickable file:line links in terminal (opens in your editor)
- JSON cache to avoid repeated API calls
- Colored status output for quick scanning

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
# Scan current directory (shows only updates)
chartup .

# Scan specific path
chartup /path/to/helm/charts

# Show all items including up-to-date and skipped
chartup --verbose .

# Force fresh lookups and update cache
chartup --refresh .

# Specify editor for links (auto-detects from $EDITOR)
chartup --editor vscode .
```

## Options

| Flag | Description |
|------|-------------|
| `--verbose` | Show all items (default: only updates) |
| `--refresh` | Refresh cache with fresh lookups |
| `--editor` | Editor for file links: `vscode`, `cursor`, `idea`, `sublime`, `zed`, `none` |
| `--version` | Show version |
| `--help` | Show help |

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

## Dockerfile Scanning

Scans Dockerfiles for `FROM` instructions and extracts base images.

**Supported filenames:**
- `Dockerfile`
- `*.dockerfile` (e.g., `app.dockerfile`)
- `Dockerfile.*` (e.g., `Dockerfile.prod`)

**Features:**
- Multi-stage builds (all `FROM` instructions)
- ARG variable resolution (`$VAR`, `${VAR}`, `${VAR:-default}`)
- Skips `scratch`, stage aliases, and unresolvable variables

## Example Output

```
DOCKER IMAGES - 3 updates
════════════════════════════════════════════════════════════════════════════════
┌─────────────────────────────────┬───────────────┬─────────┬──────────────┐
│ LOCATION                        │ IMAGE         │ CURRENT │ LATEST       │
├─────────────────────────────────┼───────────────┼─────────┼──────────────┤
│ 1_setup/6_trino/values.yaml:7   │ trinodb/trino │ 410     │ 479          │
│ 1_setup/6_trino/values.yaml:163 │ busybox       │ 1.28    │ 1.37.0-glibc │
└─────────────────────────────────┴───────────────┴─────────┴──────────────┘

HELM CHARTS - 1 updates
════════════════════════════════════════════════════════════════════════════════
┌────────────────────────────────┬───────┬─────────┬────────┐
│ LOCATION                       │ CHART │ CURRENT │ LATEST │
├────────────────────────────────┼───────┼─────────┼────────┤
│ 1_setup/6_trino/Chart.yaml     │ trino │ 0.8.0   │ 1.41.0 │
└────────────────────────────────┴───────┴─────────┴────────┘

╭────────────────────────╮
│        SUMMARY         │
├───────────────────┬────┤
│ Updates available │ 4  │
│ Up to date        │ 2  │
│ Skipped           │ 1  │
├───────────────────┼────┤
│ Total             │ 7  │
╰───────────────────┴────╯

Hint: Run with --verbose to show all 7 items
```

## License

MIT
