# Changelog

## [0.1.1] - 2026-08-08

### Added

- Background job infrastructure for long-running tools. `cleaner_run`, `cleaner_purge`, `recycle_empty`, `wu_cache_clear`, `dism_cleanup`, `winsxs_superseded`, and `cleanup_all` now return a `job_id` immediately and run in the background; `cleaner_poll` retrieves status, incremental progress, and the final result, and `cleaner_jobs` lists recent jobs.
- Per-target progress reporting in the cache cleaner (`cleaner.RunProgress`) with per-section progress for the `cleanup_all` dev-cache stage.

## [0.1.0] - 2026-08-07

### Added

- Initial MCP server for Windows system cleanup and developer-environment management.
- Cache cleaner (`cleaner_scan`, `cleaner_run`, `cleaner_purge`) across 10 dev-cache categories, with per-command timeouts on purge.
- System info (`disk_usage`, `dir_sizes`, `hibernate_status`, `set_hibernate`, `dism_cleanup`, `check_admin`).
- Language manager (`lang_list`, `lang_detect`, `lang_manage`) via uv/winget/choco.
- Recycle bin, thumbnail cache and temp folder tools (`recycle_info`, `recycle_empty`, `thumbnail_info`, `thumbnail_clear`, `temp_info`, `temp_clean`).
- System ops (`startup_inventory`, `pagefile_info`, `wu_cache_info`, `wu_cache_clear`, `winsxs_superseded`).
- Diagnostics (`system_report`, `cleanup_all` aggregate with dry-run preview).
- Multi-client MCP configuration reference (`docs/reference/mcp-client-configs.md`).
- Roadmap documenting the planned GUI so users and AI can see and interact with cleanup state.
