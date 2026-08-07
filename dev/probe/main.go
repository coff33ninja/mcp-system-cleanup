package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"systemcleanup/mcp/internal/server"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	s := server.New("test")

	ct, st := mcp.NewInMemoryTransports()
	go func() {
		if _, err := s.Connect(ctx, st, nil); err != nil {
			fmt.Println("server connect err:", err)
		}
	}()

	c := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "1.0"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	if err != nil {
		fmt.Println("client connect err:", err)
		return
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		fmt.Println("list err:", err)
		return
	}
	fmt.Printf("TOOLS=%d\n", len(tools.Tools))
	for _, t := range tools.Tools {
		fmt.Printf("  %s\n", t.Name)
	}

	for _, name := range []string{"disk_usage", "hibernate_status", "check_admin"} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name})
		if err != nil {
			fmt.Printf("%s err: %v\n", name, err)
			continue
		}
		fmt.Printf("%s => %s\n", name, trunc(res))
	}

	scan, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "cleaner_scan", Arguments: map[string]any{"min_size_mb": 50}})
	if err != nil {
		fmt.Println("cleaner_scan err:", err)
	} else {
		fmt.Printf("cleaner_scan (min 50MB) => %s\n", trunc(scan))
	}

	langs, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "lang_detect"})
	if err != nil {
		fmt.Println("lang_detect err:", err)
	} else {
		fmt.Printf("lang_detect => %s\n", trunc(langs))
	}

	for _, name := range []string{"recycle_info", "thumbnail_info", "temp_info", "pagefile_info", "startup_inventory", "wu_cache_info", "system_report"} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name})
		if err != nil {
			fmt.Printf("%s err: %v\n", name, err)
			continue
		}
		fmt.Printf("%s => %s\n", name, trunc(res))
	}

	dry, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "cleanup_all", Arguments: map[string]any{"dry_run": true}})
	if err != nil {
		fmt.Println("cleanup_all (dry) err:", err)
	} else {
		fmt.Printf("cleanup_all (dry) => %s\n", trunc(dry))
	}
}

func trunc(res *mcp.CallToolResult) string {
	if res == nil {
		return "<nil>"
	}
	s := fmt.Sprintf("%v", res)
	if len(s) > 220 {
		return s[:220] + "..."
	}
	return s
}
