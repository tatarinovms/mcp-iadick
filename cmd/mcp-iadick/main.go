package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/tatarinovms/mcp-iadick/internal/rclone"
	"github.com/tatarinovms/mcp-iadick/internal/server"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		transport   = flag.String("transport", "stdio", "Transport protocol: 'stdio', 'sse', or 'http' (streamable-http)")
		addr        = flag.String("addr", ":8080", "Network address to listen on for 'sse' and 'http' modes (e.g. ':8080' or '0.0.0.0:8080')")
		baseURL     = flag.String("base-url", "", "Base URL for SSE server (e.g. 'http://localhost:8080', defaults to http://localhost:<port>)")
		remote      = flag.String("remote", "", "rclone remote name (default: 'yandex' or RCLONE_REMOTE env var)")
		rclonePath  = flag.String("rclone-path", "", "Path to rclone binary (default: auto-detected or RCLONE_PATH env var)")
		showVersion = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("mcp-iadick version %s (commit: %s, date: %s)\n", version, commit, date)
		return
	}

	client, err := rclone.NewClient(*remote, *rclonePath)
	if err != nil {
		log.Fatalf("Failed to initialize rclone client: %v", err)
	}

	mcpSrv := server.NewServer(client)

	switch strings.ToLower(*transport) {
	case "stdio":
		if err := mcpserver.ServeStdio(mcpSrv); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}

	case "sse":
		bURL := *baseURL
		if bURL == "" {
			port := "8080"
			if strings.Contains(*addr, ":") {
				parts := strings.Split(*addr, ":")
				if parts[len(parts)-1] != "" {
					port = parts[len(parts)-1]
				}
			}
			bURL = fmt.Sprintf("http://localhost:%s", port)
		}

		log.Printf("Starting MCP Yandex Disk SSE server %s on %s (base URL: %s)...", version, *addr, bURL)
		log.Printf("SSE endpoint: %s/sse", bURL)
		log.Printf("Remote: %s:", client.Remote)

		sseServer := mcpserver.NewSSEServer(
			mcpSrv,
			mcpserver.WithBaseURL(bURL),
		)
		if err := sseServer.Start(*addr); err != nil {
			log.Fatalf("SSE server failed: %v", err)
		}

	case "http", "streamable-http":
		log.Printf("Starting MCP Yandex Disk Streamable HTTP server %s on %s...", version, *addr)
		log.Printf("HTTP endpoint: http://localhost%s/mcp", *addr)
		log.Printf("Remote: %s:", client.Remote)

		httpServer := mcpserver.NewStreamableHTTPServer(mcpSrv)
		if err := httpServer.Start(*addr); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown transport %q. Allowed: stdio, sse, http\n", *transport)
		os.Exit(1)
	}
}
