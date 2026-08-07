package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"systemcleanup/mcp/internal/cleaner"
	"systemcleanup/mcp/internal/recycle"
	"systemcleanup/mcp/internal/sysinfo"
	"systemcleanup/mcp/internal/sysops"
)

// ============================ Recycle bin ============================

type RecycleEmptyArgs struct {
	DryRun     bool `json:"dry_run,omitempty"`
	TimeoutSec int  `json:"timeout_sec,omitempty"`
}

func recycleInfoHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"recycle_bin": recycle.Info()}, nil
}

func recycleEmptyHandler(ctx context.Context, req *mcp.CallToolRequest, args RecycleEmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"result": recycle.Empty(ctx, args.DryRun, args.TimeoutSec)}, nil
}

// ============================ Thumbnails & temp ============================

type DryRunArgs struct {
	DryRun bool `json:"dry_run,omitempty"`
}

func thumbnailInfoHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"thumbnails": recycle.Thumbnails()}, nil
}

func thumbnailClearHandler(ctx context.Context, req *mcp.CallToolRequest, args DryRunArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"result": recycle.ClearThumbnails(args.DryRun)}, nil
}

func tempInfoHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"temp": recycle.Temp()}, nil
}

func tempCleanHandler(ctx context.Context, req *mcp.CallToolRequest, args DryRunArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"result": recycle.CleanTemp(args.DryRun)}, nil
}

// ============================ System ops ============================

func startupHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	items := sysops.StartupInventory()
	return &mcp.CallToolResult{}, map[string]any{"startup_items": items, "count": len(items)}, nil
}

func pagefileHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"pagefiles": sysops.Pagefiles()}, nil
}

type WUCacheArgs struct {
	DryRun     bool `json:"dry_run,omitempty"`
	TimeoutSec int  `json:"timeout_sec,omitempty"`
}

func wuCacheInfoHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"wu_cache": sysops.WUCache()}, nil
}

func wuCacheClearHandler(ctx context.Context, req *mcp.CallToolRequest, args WUCacheArgs) (*mcp.CallToolResult, any, error) {
	if !isAdmin() {
		return nil, nil, fmt.Errorf("wu_cache_clear: requires administrator privileges")
	}
	return &mcp.CallToolResult{}, map[string]any{"result": sysops.ClearWUCache(ctx, args.DryRun, args.TimeoutSec)}, nil
}

func winsxsHandler(ctx context.Context, req *mcp.CallToolRequest, args DISMArgs) (*mcp.CallToolResult, any, error) {
	if !isAdmin() {
		return nil, nil, fmt.Errorf("winsxs_superseded: requires administrator privileges")
	}
	res := sysinfo.DISMResetBase(ctx, args.TimeoutSec)
	if res.Error != "" {
		return &mcp.CallToolResult{}, map[string]any{"ok": false, "error": res.Error, "elapsed_ms": res.ElapsedMs, "output": res.Output}, nil
	}
	return &mcp.CallToolResult{}, map[string]any{"ok": true, "elapsed_ms": res.ElapsedMs, "output": res.Output}, nil
}

// ============================ Diagnostics ============================

func systemReportHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	startup := sysops.StartupInventory()
	return &mcp.CallToolResult{}, map[string]any{
		"os":            sysinfo.OSVersion(),
		"memory":        sysinfo.Memory(),
		"volumes":       sysinfo.Volumes(),
		"admin":         isAdmin(),
		"hibernate":     sysinfo.Hibernate(),
		"recycle_bin":   recycle.Info(),
		"thumbnails":    recycle.Thumbnails(),
		"temp":          recycle.Temp(),
		"pagefiles":     sysops.Pagefiles(),
		"wu_cache":      sysops.WUCache(),
		"startup_count": len(startup),
		"startup_items": startup,
	}, nil
}

type CleanupAllArgs struct {
	DryRun     bool `json:"dry_run,omitempty"`
	TimeoutSec int  `json:"timeout_sec,omitempty"`
}

// cleanupAllHandler runs every cleanup in sequence. Admin-only sections
// (Windows Update cache, WinSxS) are skipped when not elevated.
func cleanupAllHandler(ctx context.Context, req *mcp.CallToolRequest, args CleanupAllArgs) (*mcp.CallToolResult, any, error) {
	report := map[string]any{"dry_run": args.DryRun}
	report["dev_caches"] = cleaner.Run(cleaner.Options{DryRun: args.DryRun})
	report["recycle_bin"] = recycle.Empty(ctx, args.DryRun, args.TimeoutSec)
	report["thumbnails"] = recycle.ClearThumbnails(args.DryRun)
	report["temp"] = recycle.CleanTemp(args.DryRun)
	if !isAdmin() {
		report["wu_cache"] = map[string]any{"error": "requires administrator privileges"}
		report["winsxs"] = map[string]any{"error": "requires administrator privileges"}
		return &mcp.CallToolResult{}, report, nil
	}
	report["wu_cache"] = sysops.ClearWUCache(ctx, args.DryRun, args.TimeoutSec)
	if args.DryRun {
		report["winsxs"] = map[string]any{"dry_run": true}
	} else {
		report["winsxs"] = sysinfo.DISMResetBase(ctx, args.TimeoutSec)
	}
	return &mcp.CallToolResult{}, report, nil
}

// ============================ Registrations ============================

// registerOpsTools wires the recycle bin, system ops, diagnostics and
// all-in-one cleanup tools into the server.
func registerOpsTools(server *mcp.Server) {
	addToolClean(server, &mcp.Tool{
		Name:        "recycle_info",
		Description: "Report the total size and item count of the recycle bin on every drive.",
	}, safeHandler("recycle_info", recycleInfoHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "recycle_empty",
		Description: "Empty the recycle bin on every drive. Pass dry_run=true to preview the current contents first. Optional timeout_sec (default 120).",
	}, safeHandler("recycle_empty", recycleEmptyHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "thumbnail_info",
		Description: "Report the size of the Explorer thumbnail and icon cache (thumbcache_*.db / iconcache_*.db).",
	}, safeHandler("thumbnail_info", thumbnailInfoHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "thumbnail_clear",
		Description: "Delete the Explorer thumbnail and icon cache. Files locked by Explorer are reported and left in place; caches rebuild on demand.",
	}, safeHandler("thumbnail_clear", thumbnailClearHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "temp_info",
		Description: "Report the size of the user and system temp folders.",
	}, safeHandler("temp_info", tempInfoHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "temp_clean",
		Description: "Remove the contents of the user and system temp folders. Locked files are skipped and reported.",
	}, safeHandler("temp_clean", tempCleanHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "startup_inventory",
		Description: "List startup entries from the Run/RunOnce registry keys (HKLM + HKCU) and the user/all-users Startup folders.",
	}, safeHandler("startup_inventory", startupHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "pagefile_info",
		Description: "Report the configured page files and their current sizes.",
	}, safeHandler("pagefile_info", pagefileHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "wu_cache_info",
		Description: "Report the size of the Windows Update download cache (SoftwareDistribution\\Download).",
	}, safeHandler("wu_cache_info", wuCacheInfoHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "wu_cache_clear",
		Description: "Remove the contents of the Windows Update download cache. Requires admin; files locked by the Update service are reported.",
	}, safeHandler("wu_cache_clear", wuCacheClearHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "winsxs_superseded",
		Description: "Run DISM /Online /Cleanup-Image /StartComponentCleanup /ResetBase to permanently remove superseded Windows components from WinSxS. More aggressive than dism_cleanup. Requires admin. Optional timeout_sec (default 900).",
	}, safeHandler("winsxs_superseded", winsxsHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "system_report",
		Description: "Aggregate diagnostics: OS version, memory, disk usage, admin state, hibernation, recycle bin, thumbnail cache, temp folders, page files, Windows Update cache, and startup inventory.",
	}, safeHandler("system_report", systemReportHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "cleanup_all",
		Description: "Run every cleanup in sequence: dev caches, recycle bin, thumbnails, temp folders, Windows Update cache, and WinSxS superseded components. Admin sections are skipped when not elevated. Pass dry_run=true to preview everything.",
	}, safeHandler("cleanup_all", cleanupAllHandler))
}
