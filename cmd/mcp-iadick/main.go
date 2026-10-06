package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
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
		transport   = flag.String("transport", "stdio", "Transport protocol: 'stdio', 'http' (Streamable HTTP), 'sse', or 'dual'/'both' (HTTP + SSE)")
		addr        = flag.String("addr", ":8080", "Network address to listen on for network modes (e.g. ':8080' or '0.0.0.0:8080')")
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

	getEffectiveBaseURL := func() string {
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
		return bURL
	}

	switch strings.ToLower(*transport) {
	case "stdio":
		if err := mcpserver.ServeStdio(mcpSrv); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}

	case "http", "streamable-http":
		log.Printf("Starting MCP Yandex Disk Streamable HTTP server %s on %s...", version, *addr)
		log.Printf("Streamable HTTP endpoint: http://<host>%s/mcp (recommended for OpenCode v2)", *addr)
		log.Printf("Remote: %s:", client.Remote)

		httpServer := mcpserver.NewStreamableHTTPServer(mcpSrv)
		if err := httpServer.Start(*addr); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}

	case "sse":
		bURL := getEffectiveBaseURL()
		log.Printf("Starting MCP Yandex Disk SSE server %s on %s (base URL: %s)...", version, *addr, bURL)
		log.Printf("SSE endpoint: %s/sse (for Claude Desktop/legacy SSE clients)", bURL)
		log.Printf("Remote: %s:", client.Remote)

		sseServer := mcpserver.NewSSEServer(
			mcpSrv,
			mcpserver.WithBaseURL(bURL),
		)
		if err := sseServer.Start(*addr); err != nil {
			log.Fatalf("SSE server failed: %v", err)
		}

	case "both", "dual", "all", "network":
		bURL := getEffectiveBaseURL()
		log.Printf("Starting MCP Yandex Disk Dual server (Streamable HTTP + SSE) %s on %s...", version, *addr)
		log.Printf("Streamable HTTP endpoint: http://<host>%s/mcp (for OpenCode v2)", *addr)
		log.Printf("SSE endpoint: %s/sse (for Claude Desktop / SSE clients)", bURL)
		log.Printf("Remote: %s:", client.Remote)

		httpServer := mcpserver.NewStreamableHTTPServer(mcpSrv)
		sseServer := mcpserver.NewSSEServer(
			mcpSrv,
			mcpserver.WithBaseURL(bURL),
		)

		mux := http.NewServeMux()
		mux.Handle("/mcp", httpServer)
		mux.Handle("/sse", sseServer.SSEHandler())
		mux.Handle("/message", sseServer.MessageHandler())

		if err := http.ListenAndServe(*addr, mux); err != nil {
			log.Fatalf("Dual server failed: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown transport %q. Allowed: stdio, http, sse, dual\n", *transport)
		os.Exit(1)
	}
}
