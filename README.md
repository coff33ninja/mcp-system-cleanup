# system-cleanup-mcp

Go-based MCP server for Windows system cleanup and developer-environment management, following the architecture of `go-mcp-computer-use` (modelcontextprotocol/go-sdk, jsonschema-go schema generation, `internal/server` + `internal/actions` split).

## Tools

### Cache cleaner (`internal/cleaner`)
| Tool | Description |
|------|-------------|
| `cleaner_scan` | Dry-run scan: lists all dev cache locations with sizes. Filters: `categories`, `min_size_mb`, `older_than_days`. |
| `cleaner_run` | Deletes caches across 10 categories (Python, Node, Rust, Go, JVM, ML/AI, IDEs, Docker, browsers, DB, games...). Same filters. |
| `cleaner_purge` | Runs tool-managed purge commands (`pip cache purge`, `uv cache clean`, `npm cache clean --force`, `docker prune`, ...) with a per-command timeout so hung tools can't block the run. |

### System info (`internal/sysinfo`)
| Tool | Description |
|------|-------------|
| `disk_usage` | All drives: total/used/free + percentage. |
| `dir_sizes` | On-disk size of arbitrary files/directories. |
| `hibernate_status` | hiberfil.sys presence + size. |
| `set_hibernate` | Toggle hibernation via powercfg (frees hiberfil.sys). Requires admin. |
| `dism_cleanup` | `DISM /Online /Cleanup-Image /StartComponentCleanup` (WinSxS trim). Requires admin. |
| `check_admin` | Reports whether the server runs elevated. |

### Language manager (`internal/langmgr`)
| Tool | Description |
|------|-------------|
| `lang_list` | Supported languages + install methods (uv/winget/choco). |
| `lang_detect` | Detect installed languages and versions (parallel). |
| `lang_manage` | Install/uninstall a language via uv, winget, or choco. |

### Recycle bin, thumbnails & temp (`internal/recycle`)
| Tool | Description |
|------|-------------|
| `recycle_info` | Total size + item count of the recycle bin on every drive. |
| `recycle_empty` | Empty the recycle bin on every drive. `dry_run` previews contents first. |
| `thumbnail_info` | Size of the Explorer thumbnail/icon cache (thumbcache_*.db / iconcache_*.db). |
| `thumbnail_clear` | Delete the Explorer thumbnail/icon cache. Locked files are reported; caches rebuild on demand. |
| `temp_info` | Size of the user and system temp folders. |
| `temp_clean` | Remove temp folder contents; locked files are skipped and reported. |

### System ops (`internal/sysops`)
| Tool | Description |
|------|-------------|
| `startup_inventory` | Startup entries from Run/RunOnce registry keys (HKLM + HKCU) and the Startup folders. |
| `pagefile_info` | Configured page files and their current sizes (system-managed `?:\pagefile.sys` resolves to the system drive). |
| `wu_cache_info` | Size of the Windows Update download cache. |
| `wu_cache_clear` | Clear the Windows Update download cache. Requires admin. |
| `winsxs_superseded` | `DISM /Online /Cleanup-Image /StartComponentCleanup /ResetBase` — permanently removes superseded components. More aggressive than `dism_cleanup`. Requires admin. |

### Diagnostics (`internal/server`)
| Tool | Description |
|------|-------------|
| `system_report` | One-shot aggregate: OS version, memory, disk usage, admin state, hibernation, recycle bin, thumbnails, temp, page files, WU cache, startup inventory. |
| `cleanup_all` | Runs every cleanup in sequence: dev caches, recycle bin, thumbnails, temp, WU cache, WinSxS. Admin sections skipped when not elevated. `dry_run` previews everything. |

## Building

```powershell
cd E:\SCRIPTS\Servers\SYSTEM_CLEANUP\mcp-system-cleanup
$ver = (Get-Content VERSION -Raw).Trim()
go build -ldflags="-X main.Version=$ver" -o mcp-server.exe ./cmd/mcp-server
```

The canonical version lives in `VERSION` at the repo root. `launch.ps1` builds with this version automatically.

## Development & releases

Local gates and the release pipeline live in `scripts/` (mirrors go-mcp-computer-use's setup):

```powershell
.\scripts\lint.ps1            # go vet + build with version injection
.\scripts\build.ps1           # version-injected build (-Release for stripped)
```

To cut a release: bump `VERSION`, add a `docs/meta/CHANGELOG.md` entry, then run `.\scripts\push-and-release.ps1` — it commits, tags, pushes, waits for the GitHub Release workflow, and swaps in the CI-built binary. CI (`ci.yml`) runs vet + build on every push/PR; the tag-triggered `release.yml` builds and publishes `mcp-server.exe` + SHA256 + `launch.ps1`. See [docs/ci-cd-pipeline.md](docs/ci-cd-pipeline.md) and [docs/reference/versioning-strategy.md](docs/reference/versioning-strategy.md).

## Registering with AI assistants

`mcp-server.exe` is a stdio MCP server and works with any client that supports local MCP servers. Most clients share the same JSON shape:

```json
{
  "mcpServers": {
    "system-cleanup": {
      "command": "E:\\SCRIPTS\\Servers\\SYSTEM_CLEANUP\\mcp-system-cleanup\\mcp-server.exe"
    }
  }
}
```

Some clients use different top-level keys, and several provide CLI add commands (`claude mcp add`, `gemini mcp add`, `opencode mcp add`). Prefer the CLI where available — it handles schema validation and scoping. Per-client config files, JSON examples, a quick-reference table, and troubleshooting are in [docs/reference/mcp-client-configs.md](docs/reference/mcp-client-configs.md).

Quick reference:

| Client | Config file | Top-level key | CLI add |
|--------|-------------|---------------|---------|
| Claude Desktop | `%APPDATA%\Claude\claude_desktop_config.json` | `mcpServers` | — |
| Claude Code | `.mcp.json` / `~/.claude.json` | `mcpServers` | `claude mcp add system-cleanup -- E:\...\mcp-server.exe` |
| OpenCode | `opencode.json` / `~/.config/opencode/opencode.json` | `mcp` | `opencode mcp add` |
| Cursor | `.cursor/mcp.json` / `~/.cursor/mcp.json` | `mcpServers` | — |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `mcpServers` | — |
| VS Code Copilot | `.vscode/mcp.json` / `settings.json` | `servers` | — |
| Continue.dev | `~/.continue/config.json` | `mcpServers` (array) | — |
| Cline | `%APPDATA%\Code\...\cline_mcp_settings.json` | `mcpServers` | — |
| Gemini CLI | `.gemini/settings.json` | `mcpServers` | `gemini mcp add system-cleanup -- E:\...\mcp-server.exe` |
| Roo Code | `mcp_settings.json` / `.roo/mcp.json` | `mcpServers` | — |
| Zed | `%APPDATA%\zed\settings.json` | `context_servers` | — |
| JetBrains IDEs | `%APPDATA%\JetBrains\AIAssistant\mcp.json` | `mcpServers` | — |
| Emacs | `~/.emacs.d/init.el` (`mcp-hub-servers`) | plist | — |
| Sourcegraph Cody | `settings.json` under `cody.mcpServers` | `cody.mcpServers` | — |

### OpenCode

OpenCode uses a different schema than most clients — top-level `mcp` key, array command, `environment` for env vars. Add to `opencode.json` (or `~/.config/opencode/opencode.json`):

```jsonc
{
  "mcp": {
    "system-cleanup": {
      "type": "local",
      "command": ["E:\\SCRIPTS\\Servers\\SYSTEM_CLEANUP\\mcp-system-cleanup\\mcp-server.exe"],
      "enabled": true
    }
  }
}
```

Alternatively run `opencode mcp add` and fill in the interactive prompt. Verify with `opencode mcp list`.

## Roadmap

This is only the beginning. The MCP server is the foundation; the next step is a GUI built around it so the state of the machine — what's taking space, what could be reclaimed, what was cleaned — can be **seen and interacted with**:

- **For users** — a visual dashboard over the same data the tools already return: per-category cache sizes, recycle bin / temp / WU cache / WinSxS totals, startup inventory, and one-click actions with dry-run previews.
- **For AI** — the GUI and its actions exposed the same way the MCP tools are, so agents can inspect, plan, and perform cleanup with the same visibility a human has.

Every tool already returns structured JSON (`system_report`, `cleanup_all`, and the individual info/clear tools), so a GUI or agent front-end can be built on top of the server without changing it.

## Notes

- `cleaner_run`, `recycle_empty`, `thumbnail_clear`, `temp_clean`, and `cleanup_all` are safety-sensitive tools: always use `dry_run: true` first to preview, then rerun without it.
- `dism_cleanup`, `winsxs_superseded`, `set_hibernate`, `wu_cache_clear`, and `lang_manage` (except `uv`) require the server to be running elevated.
- Admin detection uses the `\\.\pipe\srvsvc` probe (matching go-mcp-computer-use's approach).
- Dev probe client lives in `dev/probe` (in-memory transport smoke test).
