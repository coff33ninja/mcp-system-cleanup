package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"runtime"

	jsonschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"systemcleanup/mcp/internal/cleaner"
	"systemcleanup/mcp/internal/langmgr"
	"systemcleanup/mcp/internal/sysinfo"
)

// cleanNullableTypes strips nullable union types from a generated JSON Schema.
// The Go jsonschema library emits "type": ["null", "X"] for pointer types,
// which the opencode MCP client cannot serialize. See the reference server
// go-mcp-computer-use for the same workaround.
func cleanNullableTypes(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) > 0 {
		nonNull := make([]string, 0, len(s.Types))
		for _, t := range s.Types {
			if t != "null" {
				nonNull = append(nonNull, t)
			}
		}
		if len(nonNull) == 1 {
			s.Type = nonNull[0]
			s.Types = nil
		} else if len(nonNull) > 1 {
			s.Types = nonNull
		} else {
			s.Types = nil
		}
	}
	for _, p := range s.Properties {
		cleanNullableTypes(p)
	}
	if s.Items != nil {
		cleanNullableTypes(s.Items)
	}
	if s.AdditionalProperties != nil {
		cleanNullableTypes(s.AdditionalProperties)
	}
	for _, v := range s.Defs {
		cleanNullableTypes(v)
	}
}

// addToolClean auto-generates the JSON Schema from the Go In type and strips
// nullable union types before registering, fixing the opencode MCP client bug.
func addToolClean[In, Out any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	rt := reflect.TypeFor[In]()
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() == reflect.Struct && tool.InputSchema == nil {
		schema, err := jsonschema.ForType(rt, &jsonschema.ForOptions{})
		if err == nil {
			schema.Schema = ""
			cleanNullableTypes(schema)
			tool.InputSchema = schema
		}
	}
	mcp.AddTool(server, tool, handler)
}

func safeHandler[Args any](name string, fn func(ctx context.Context, req *mcp.CallToolRequest, args Args) (*mcp.CallToolResult, any, error)) func(ctx context.Context, req *mcp.CallToolRequest, args Args) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args Args) (result *mcp.CallToolResult, payload any, err error) {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				slog.Error("panic in tool handler", "tool", name, "panic", fmt.Sprintf("%v", r), "stack", string(buf[:n]))
				result = nil
				payload = nil
				err = fmt.Errorf("panic in %s: %v", name, r)
			}
		}()
		return fn(ctx, req, args)
	}
}

func isAdmin() bool {
	f, err := os.Open(`\\.\pipe\srvsvc`)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// ============================ Cleaner tools ============================

type CleanerScanArgs struct {
	Categories    []int `json:"categories,omitempty"`
	MinSizeMB     int   `json:"min_size_mb,omitempty"`
	OlderThanDays int   `json:"older_than_days,omitempty"`
}

func cleanerScanHandler(ctx context.Context, req *mcp.CallToolRequest, args CleanerScanArgs) (*mcp.CallToolResult, any, error) {
	opts := cleaner.Options{
		Categories:    args.Categories,
		MinSizeMB:     args.MinSizeMB,
		OlderThanDays: args.OlderThanDays,
	}
	targets := cleaner.Scan(opts)
	var total float64
	for _, t := range targets {
		total += t.SizeMB
	}
	return &mcp.CallToolResult{}, map[string]any{
		"categories": cleaner.Categories(),
		"targets":    targets,
		"count":      len(targets),
		"total_mb":   total,
		"dry_run":    true,
	}, nil
}

type CleanerRunArgs struct {
	DryRun        bool  `json:"dry_run,omitempty"`
	Categories    []int `json:"categories,omitempty"`
	MinSizeMB     int   `json:"min_size_mb,omitempty"`
	OlderThanDays int   `json:"older_than_days,omitempty"`
}

func cleanerRunHandler(ctx context.Context, req *mcp.CallToolRequest, args CleanerRunArgs) (*mcp.CallToolResult, any, error) {
	opts := cleaner.Options{
		DryRun:        args.DryRun,
		Categories:    args.Categories,
		MinSizeMB:     args.MinSizeMB,
		OlderThanDays: args.OlderThanDays,
	}
	report := cleaner.Run(opts)
	return &mcp.CallToolResult{}, map[string]any{
		"freed_mb":   report.FreedMB,
		"skipped_mb": report.SkippedMB,
		"total_mb":   report.TotalMB,
		"summary":    report.Summary(),
		"results":    report.Results,
	}, nil
}

type CleanerPurgeArgs struct {
	DryRun          bool `json:"dry_run,omitempty"`
	PurgeTimeoutSec int  `json:"purge_timeout_sec,omitempty"`
}

func cleanerPurgeHandler(ctx context.Context, req *mcp.CallToolRequest, args CleanerPurgeArgs) (*mcp.CallToolResult, any, error) {
	opts := cleaner.Options{DryRun: args.DryRun, PurgeTimeoutSec: args.PurgeTimeoutSec}
	results := cleaner.Purge(ctx, opts)
	return &mcp.CallToolResult{}, map[string]any{"results": results}, nil
}

// ============================ System info tools ============================

type EmptyArgs struct{}

func volumesHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"volumes": sysinfo.Volumes()}, nil
}

type DirSizesArgs struct {
	Paths []string `json:"paths"`
}

func dirSizesHandler(ctx context.Context, req *mcp.CallToolRequest, args DirSizesArgs) (*mcp.CallToolResult, any, error) {
	if len(args.Paths) == 0 {
		return nil, nil, fmt.Errorf("dir_sizes: paths is required")
	}
	return &mcp.CallToolResult{}, map[string]any{"dirs": sysinfo.DirSizes(args.Paths)}, nil
}

func hibernateHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"hibernate": sysinfo.Hibernate()}, nil
}

type SetHibernateArgs struct {
	On bool `json:"on"`
}

func setHibernateHandler(ctx context.Context, req *mcp.CallToolRequest, args SetHibernateArgs) (*mcp.CallToolResult, any, error) {
	out, err := sysinfo.SetHibernate(args.On)
	if err != nil {
		return nil, nil, fmt.Errorf("set_hibernate: %w", err)
	}
	return &mcp.CallToolResult{}, map[string]any{"ok": true, "output": out, "hibernate": sysinfo.Hibernate()}, nil
}

type DISMArgs struct {
	TimeoutSec int `json:"timeout_sec,omitempty"`
}

func dismHandler(ctx context.Context, req *mcp.CallToolRequest, args DISMArgs) (*mcp.CallToolResult, any, error) {
	if !isAdmin() {
		return nil, nil, fmt.Errorf("dism_cleanup: requires administrator privileges")
	}
	res := sysinfo.DISMCleanup(ctx, args.TimeoutSec)
	if res.Error != "" {
		return &mcp.CallToolResult{}, map[string]any{"ok": false, "error": res.Error, "elapsed_ms": res.ElapsedMs, "output": res.Output}, nil
	}
	return &mcp.CallToolResult{}, map[string]any{"ok": true, "elapsed_ms": res.ElapsedMs, "output": res.Output}, nil
}

func adminHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"admin": isAdmin()}, nil
}

// ============================ Language manager tools ============================

func langsHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	langs := langmgr.All()
	type langOut struct {
		Name    string   `json:"name"`
		Methods []string `json:"methods"`
	}
	out := make([]langOut, 0, len(langs))
	for _, l := range langs {
		out = append(out, langOut{Name: l.Name, Methods: l.Methods})
	}
	return &mcp.CallToolResult{}, map[string]any{"languages": out}, nil
}

func detectHandler(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{}, map[string]any{"detected": langmgr.Detect()}, nil
}

type LangManageArgs struct {
	Language   string `json:"language"`
	Action     string `json:"action"` // install | uninstall
	Method     string `json:"method"` // uv | winget | choco
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

func langManageHandler(ctx context.Context, req *mcp.CallToolRequest, args LangManageArgs) (*mcp.CallToolResult, any, error) {
	if args.Action != "install" && args.Action != "uninstall" {
		return nil, nil, fmt.Errorf("lang_manage: action must be install or uninstall")
	}
	lang := langmgr.Find(args.Language)
	if lang == nil {
		return nil, nil, fmt.Errorf("lang_manage: unknown language %q", args.Language)
	}
	if args.Method == "" {
		args.Method = lang.Methods[0]
	}
	if !isAdmin() && args.Method != "uv" {
		return nil, nil, fmt.Errorf("lang_manage: %s requires administrator privileges", args.Method)
	}
	results := langmgr.Manage(ctx, lang, args.Action, args.Method, args.TimeoutSec)
	return &mcp.CallToolResult{}, map[string]any{
		"language": lang.Name,
		"action":   args.Action,
		"method":   args.Method,
		"results":  results,
	}, nil
}

// ============================ Server ============================

// New constructs the MCP server and registers all tools.
func New(version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "system-cleanup-mcp",
		Version: version,
	}, nil)

	addToolClean(server, &mcp.Tool{
		Name:        "cleaner_scan",
		Description: "Scan all development cache locations and report their sizes without deleting anything (dry run). Filters: categories (1-10), min_size_mb, older_than_days.",
	}, safeHandler("cleaner_scan", cleanerScanHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "cleaner_run",
		Description: "Delete development caches across 10 categories (pip, npm, uv, cargo, Go, Rust, HuggingFace, JetBrains, VS Code, Docker, browsers, etc.). Use dry_run=true to preview. Filters: categories, min_size_mb, older_than_days.",
	}, safeHandler("cleaner_run", cleanerRunHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "cleaner_purge",
		Description: "Run tool-managed cache purge commands (pip cache purge, uv cache clean, npm cache clean --force, docker prune, etc.) with a per-command timeout so hung tools cannot block the run. dry_run=true previews which tools are available.",
	}, safeHandler("cleaner_purge", cleanerPurgeHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "disk_usage",
		Description: "List all drives with total, used, free bytes and used percentage.",
	}, safeHandler("disk_usage", volumesHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "dir_sizes",
		Description: "Compute the on-disk size of arbitrary files or directories. paths is an array of absolute paths.",
	}, safeHandler("dir_sizes", dirSizesHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "hibernate_status",
		Description: "Report whether the hibernation file (hiberfil.sys) exists and its size.",
	}, safeHandler("hibernate_status", hibernateHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "set_hibernate",
		Description: "Enable or disable Windows hibernation via powercfg. Disabling deletes hiberfil.sys and frees disk space. Requires admin.",
	}, safeHandler("set_hibernate", setHibernateHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "dism_cleanup",
		Description: "Run DISM /Online /Cleanup-Image /StartComponentCleanup to trim the Windows component store (WinSxS). Requires admin and can take several minutes. Optional timeout_sec (default 900).",
	}, safeHandler("dism_cleanup", dismHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "check_admin",
		Description: "Report whether this MCP server is running with administrator privileges.",
	}, safeHandler("check_admin", adminHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "lang_list",
		Description: "List all supported languages and the install methods available for each (uv, winget, choco).",
	}, safeHandler("lang_list", langsHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "lang_detect",
		Description: "Detect which programming languages are currently installed on this machine and their versions.",
	}, safeHandler("lang_detect", detectHandler))

	addToolClean(server, &mcp.Tool{
		Name:        "lang_manage",
		Description: "Install or uninstall a programming language using uv, winget, or choco. action: install|uninstall. method defaults to the first supported for the language. Requires admin except uv.",
	}, safeHandler("lang_manage", langManageHandler))

	registerOpsTools(server)

	return server
}
