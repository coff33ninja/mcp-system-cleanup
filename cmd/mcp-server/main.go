package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"systemcleanup/mcp/internal/server"
)

var Version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: nil})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(Version)

	slog.Info("starting system-cleanup-mcp on stdio")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 8192)
				n := runtime.Stack(buf, false)
				slog.Error("FATAL PANIC (server crashed)", "panic", fmt.Sprintf("%v", r), "stack", string(buf[:n]))
				os.Exit(2)
			}
		}()
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			slog.Error("server exited", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	os.Exit(0)
}
