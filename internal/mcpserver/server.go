// Package mcpserver implements an MCP (Model Context Protocol) server that
// exposes Frisco API operations as tools consumable by AI agents.
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

// serverVersion is reported to MCP clients as the server implementation version.
const serverVersion = "0.1.0"

// New builds an MCP server exposing Frisco tools.
func New() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "frisco",
		Version: serverVersion,
	}, nil)

	registerCartAndProductsTools(server)
	registerOrdersAndReservationTools(server)
	registerAccountSessionAuthTools(server)

	return server
}
