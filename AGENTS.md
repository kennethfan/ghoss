# PROJECT KNOWLEDGE BASE

**Generated:** 2026-07-08 11:27
**Commit:** `c3960d6`
**Branch:** `dev`

## OVERVIEW

GHoss — GitHub Object Storage Tool. Go CLI that uses GitHub repos as object storage via `go-github` API. Uses cobra/viper for CLI/config.

## STRUCTURE

```
./
├── cmd/ghoss/       # CLI entry (main.go, 551 loc — oversized)
├── internal/
│   ├── config/      # Config struct + viper load/save
│   ├── github/      # GitHub API client wrapper
│   └── storage/     # Local file ops (walk, validate, normalize)
├── pkg/models/      # Result structs (Upload, Download, List, Delete)
├── bin/             # Built binary
├── go.mod
└── TECHNICAL_DESIGN.md
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add/modify CLI command | `cmd/ghoss/main.go` | All cobra commands defined here |
| Change GitHub API logic | `internal/github/client.go` | Upload, download, list, delete, URL |
| Change config format | `internal/config/config.go` | Struct + viper marshal/unmarshal |
| File path utilities | `internal/storage/storage.go` | Normalize, walk, validate |
| Add shared model | `pkg/models/file.go` | Data types only, no logic |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main()` | func | `cmd/ghoss/main.go:526` | Entry point, wires cobra commands |
| `rootCmd` | var | `cmd/ghoss/main.go:41` | Cobra root command |
| `initCmd` | var | `cmd/ghoss/main.go:56` | `ghoss init` — config setup |
| `uploadCmd` | var | `cmd/ghoss/main.go:121` | `ghoss upload` |
| `downloadCmd` | var | `cmd/ghoss/main.go:279` | `ghoss download` |
| `lsCmd` | var | `cmd/ghoss/main.go:349` | `ghoss ls` |
| `rmCmd` | var | `cmd/ghoss/main.go:395` | `ghoss rm` |
| `urlCmd` | var | `cmd/ghoss/main.go:437` | `ghoss url` |
| `uploadFile()` | func | `cmd/ghoss/main.go:163` | Single file upload logic |
| `uploadDirectory()` | func | `cmd/ghoss/main.go:238` | Recursive dir upload |
| `downloadFile()` | func | `cmd/ghoss/main.go:326` | Download + write to disk |
| `parseFileSize()` | func | `cmd/ghoss/main.go:503` | "100MB" → int64 parser |
| `Config` | struct | `internal/config/config.go:11` | App config (Github, Storage, Cache) |
| `GithubConfig` | struct | `internal/config/config.go:18` | Token, Owner, Repo, Branch |
| `LoadConfig()` | func | `internal/config/config.go:56` | Viper read + env override |
| `SaveConfig()` | func | `internal/config/config.go:108` | Viper write + chmod 600 |
| `Client` | struct | `internal/github/client.go:15` | GitHub API wrapper |
| `NewClient()` | func | `internal/github/client.go:24` | OAuth2 + go-github init |
| `UploadFile()` | method | `internal/github/client.go:44` | PUT /contents/{path} |
| `DownloadFile()` | method | `internal/github/client.go:62` | GET download URL → raw bytes |
| `ListFiles()` | method | `internal/github/client.go:101` | GET /contents/{path} |
| `DeleteFile()` | method | `internal/github/client.go:110` | GET SHA → DELETE |
| `GetFileURL()` | method | `internal/github/client.go:133` | Download URL from metadata |
| `File` | struct | `internal/storage/storage.go:11` | Local file info |
| `ListDirectory()` | func | `internal/storage/storage.go:42` | Recursive file walk |
| `NormalizePath()` | func | `internal/storage/storage.go:70` | Trim slash, backslash→slash |
| `File` | struct | `pkg/models/file.go:6` | Remote file metadata |
| `UploadResult` | struct | `pkg/models/file.go:17` | Upload operation result |
| `DownloadResult` | struct | `pkg/models/file.go:25` | Download operation result |

## CONVENTIONS

- **Go std project layout**: `cmd/` entry, `internal/` private packages, `pkg/` public
- **No error wrapping library**: Uses `fmt.Errorf("...: %w", err)` directly
- **No tests yet**: No `_test.go` files anywhere
- **No CI/CD**: No workflows, Makefile, or Dockerfile
- **Config security**: `chmod 600` on config file containing token
- **Single binary**: No multi-platform builds configured

## ANTI-PATTERNS (THIS PROJECT)

- `cmd/ghoss/main.go` at 551 lines — all cobra command registrations AND all CLI logic in one file. Should be split per command.
- Global `reader` variable (`bufio.NewReader`) — test-hostile, no injection possible.
- `viper.Set()` before `WriteConfig()` in `SaveConfig` — viper accumulates state across calls.
- `uploadDirectory()` silently skips dirs in recursive mode — `if file.IsDir { continue }` with no log/warning.
- `ListFiles()` dereferences `*file.Type` and `*file.Name` without nil check — potential nil ptr panic.
- No input validation on token/owner/repo format beyond empty check.

## COMMANDS

```bash
go build -o ghoss cmd/ghoss/main.go   # Build
./ghoss init                           # Init config
./ghoss upload <file>                  # Upload
./ghoss download <path>                # Download
./ghoss ls [path]                      # List
./ghoss rm <path>                      # Delete
./ghoss url <path>                     # Get URL
```

## NOTES

- Uses `go-github v17` (incompatible — pre-v17 API) — consider upgrading to `v71+`
- Single GitHub API client per command invocation (no connection pooling)
- File content goes through GitHub Contents API (Base64, max 100MB per file)
