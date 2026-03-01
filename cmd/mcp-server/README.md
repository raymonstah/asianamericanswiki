# Asian Americans Wiki MCP Server

This directory contains the Model Context Protocol (MCP) server for the Asian Americans Wiki.
It allows AI agents to interact with the wiki data directly via the MCP protocol.

## Features

- **Search**: Search for Asian Americans by name, ethnicity, or tags.
- **Get**: Retrieve detailed information about a specific entry.
- **Add**: Contribute new entries (created as drafts).
- **Update**: Edit existing entries.
- **List**: Paginate through all entries.

## Usage

The server is designed to be run as a subprocess by an MCP client (like Claude Desktop or the provided `mcp-client`).

### Running with Go

```bash
go run cmd/mcp-server/main.go
```

### Tools Provided

- `search-asian-americans`: Search by query.
- `get-human`: Get by `id` or `path`.
- `add-human`: Create a new entry.
- `update-human`: Update an entry by `id`.
- `list-humans`: List entries with `limit` and `offset`.

## Development

The server uses the project's internal `humandao` and connects directly to Firestore.
Ensure you have the necessary Google Cloud credentials configured in your environment.
