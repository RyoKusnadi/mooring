// Package fakemcp is a stand-in tool server for tests: a stdio server and a
// streamable HTTP server that wants sign-in, with its own small authorization
// server. Its test binary runs as the stdio server when MOORING_FAKE_MCP=1.
package fakemcp

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoIn struct {
	Text string `json:"text"`
}

type addIn struct {
	A int `json:"a"`
	B int `json:"b"`
}

// NewServer returns a tool server with the tools echo and add.
func NewServer() *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "fakemcp", Version: "1"}, nil)
	sdk.AddTool(s, &sdk.Tool{Name: "echo", Description: "Returns the text it was given."},
		func(_ context.Context, _ *sdk.CallToolRequest, in echoIn) (*sdk.CallToolResult, any, error) {
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: in.Text}}}, nil, nil
		})
	sdk.AddTool(s, &sdk.Tool{Name: "add", Description: "Adds two numbers."},
		func(_ context.Context, _ *sdk.CallToolRequest, in addIn) (*sdk.CallToolResult, any, error) {
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: strconv.Itoa(in.A + in.B)}}}, nil, nil
		})
	return s
}

// RunStdio serves on stdin and stdout until they close. Environment switches for tests:
//
//	FAKE_MCP_STDERR=1  write "secret=$FAKE_MCP_SECRET" and a long filler to stderr first
//	FAKE_MCP_FAIL=1    exit with status 1 after that, without serving
//	FAKE_MCP_HANG=1    start a grandchild (pid in $FAKE_MCP_PIDFILE) and never answer
//	FAKE_MCP_STARTFILE append one byte to this file at every start (counts starts)
//	FAKE_MCP_TTL=ms    set ttlMs in the tools/list result (unset: leave it out)
func RunStdio() error {
	if f := os.Getenv("FAKE_MCP_STARTFILE"); f != "" {
		if fh, err := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			_, _ = fh.WriteString("x")
			_ = fh.Close()
		}
	}
	if os.Getenv("FAKE_MCP_STDERR") == "1" || os.Getenv("FAKE_MCP_FAIL") == "1" {
		_, _ = os.Stderr.WriteString("secret=" + os.Getenv("FAKE_MCP_SECRET") + "\n" + strings.Repeat("x", 10000) + "\n")
	}
	if os.Getenv("FAKE_MCP_FAIL") == "1" {
		os.Exit(1)
	}
	if os.Getenv("FAKE_MCP_HANG") == "1" {
		child := exec.Command("sleep", "300")
		if err := child.Start(); err == nil {
			_ = os.WriteFile(os.Getenv("FAKE_MCP_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600)
		}
		time.Sleep(time.Hour)
	}
	return NewServer().Run(context.Background(), ttlTransport{&sdk.StdioTransport{}, envInt("FAKE_MCP_TTL")})
}
