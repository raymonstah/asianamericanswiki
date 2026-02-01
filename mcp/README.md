# Asian Americans Wiki MCP Server & Client

This project implements a [Model Context Protocol (MCP)](https://github.com/modelcontextprotocol/modelcontextprotocol) server and client that interacts with the [Asian Americans Wiki](https://asianamericans.wiki/) API.

## Project Structure

- **`main.go`**: The MCP Server implementation. It fetches data from the API and exposes a tool for searching Asian Americans.
- **`client/main.go`**: The MCP Client implementation. It launches the server as a subprocess and provides both a CLI and interactive interface for querying.

## Prerequisites

- [Go](https://go.dev/) (1.24 or later recommended)

## Setup & Build

1.  **Build the Server**:
    The client expects the server binary to be named `mcp-server` and located in the same directory.
    ```bash
    go build -o mcp-server main.go
    ```

2.  **Build the Client**:
    ```bash
    go build -o mcp-client client/main.go
    ```

## Usage

### 1. CLI Search (One-off)
Provide a search query as command-line arguments. The client supports natural language queries by automatically filtering out common stop words (e.g., "who", "was", "the").

```bash
./mcp-client "Who was the Korean actress on The Morning Show"
./mcp-client "famous Chinese rappers"
./mcp-client "Korean actresses"
```

### 2. Interactive Mode
Run the client without arguments to enter an interactive loop where you can ask multiple questions.

```bash
./mcp-client
# Then type your questions at the > prompt. Type 'quit' to exit.
```

## Features

-   **Natural Language Support**: The client pre-processes your input to improve search accuracy when asking full questions.
-   **Formatted Output**: Results include names and expanded descriptions (up to 600 characters) for better context.
-   **Tool-based Architecture**: Uses the `search-asian-americans` MCP tool to filter names, ethnicities, tags, and descriptions.

## How it Works

1.  The **Client** (`./mcp-client`) starts and executes the **Server** (`./mcp-server`) as a background process.
2.  They communicate over Stdin/Stdout using the MCP protocol.
3.  The Server fetches data from `https://asianamericans.wiki/api/v1/humans`.
4.  The Client sends a `CallTool` request for the `search-asian-americans` tool.
5.  The Server filters the data and returns a JSON response.
6.  The Client parses the JSON, cleans up the text, and displays it.