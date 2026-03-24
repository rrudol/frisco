package commands

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP server on stdio transport.",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcpserver.New()
			return server.Run(context.Background(), &mcp.StdioTransport{})
		},
	}
}
