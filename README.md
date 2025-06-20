# MySQL MCP Server

A Model Context Protocol (MCP) server that provides MySQL database access through standardized tools.

## Features

- **Query Execution**: Execute arbitrary SQL queries
- **Schema Inspection**: View table schemas
- **Table Listing**: List all tables in the database
- **Query Analysis**: Analyze query execution plans with optimization suggestions

## Requirements

- Go 1.21+ (developed with Go 1.23.6)
- MySQL 5.7+ or MySQL 8.0+
- Make (for build commands)

## Installation

### Option 1: Download Pre-built Binary (Recommended)

Download the latest release for your platform:

**Linux (amd64):**
```bash
curl -L https://github.com/koh/mysql-mcp-server/releases/latest/download/mysql-mcp-server-linux-amd64.tar.gz | tar xz
chmod +x mysql-mcp-server
sudo mv mysql-mcp-server /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/koh/mysql-mcp-server/releases/latest/download/mysql-mcp-server-darwin-arm64.tar.gz | tar xz
chmod +x mysql-mcp-server
mv mysql-mcp-server /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/koh/mysql-mcp-server/releases/latest/download/mysql-mcp-server-darwin-amd64.tar.gz | tar xz
chmod +x mysql-mcp-server
mv mysql-mcp-server /usr/local/bin/
```

**Windows:**
```powershell
# Download from https://github.com/koh/mysql-mcp-server/releases/latest
# Extract mysql-mcp-server-windows-amd64.zip
# Add to PATH or move mysql-mcp-server.exe to a directory in PATH
```

### Option 2: Build from Source

1. Clone the repository:
```bash
git clone https://github.com/koh/mysql-mcp-server.git
cd mysql-mcp-server
```

2. Install dependencies and build:
```bash
make build
# Or use make setup for full development setup
```

### Verify Installation

After installation, verify the server is accessible:

```bash
mysql-mcp-server --version
```

## Configuration

The server uses environment variables for MySQL connection configuration:

- `MYSQL_HOST`: MySQL server host (default: localhost)
- `MYSQL_PORT`: MySQL server port (default: 3306)
- `MYSQL_USER`: MySQL username
- `MYSQL_PASSWORD`: MySQL password
- `MYSQL_DATABASE`: Database name to connect to

You can copy `.env.example` to `.env` and modify it with your credentials:

```bash
cp .env.example .env
```

## Usage

### With Claude Desktop

Add the server to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mysql": {
      "command": "mysql-mcp-server",
      "env": {
        "MYSQL_HOST": "localhost",
        "MYSQL_PORT": "3306",
        "MYSQL_USER": "your_user",
        "MYSQL_PASSWORD": "your_password",
        "MYSQL_DATABASE": "your_database"
      }
    }
  }
}
```

### Direct Usage

Run the server directly:

```bash
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=password
export MYSQL_DATABASE=testdb
./mysql-mcp-server
```

## Available Tools

### query
Execute SELECT queries to retrieve data from MySQL database. This tool is restricted to SELECT statements only for safety. Use the `execute` tool for data modification operations.

**Parameters:**
- `query` (required): SELECT statement only
- `format` (optional): Output format - `json`, `table`, `csv`, or `markdown` (default: `table`)

**Example:**
```json
{
  "name": "query",
  "arguments": {
    "query": "SELECT * FROM users WHERE status = 'active' LIMIT 10",
    "format": "csv"
  }
}
```

### execute
Execute INSERT, UPDATE, DELETE queries with safety checks. This tool implements a two-step execution process for safety:
1. First run with `dry_run=true` to preview affected rows
2. Then run with `dry_run=false` and the confirmation token to execute

**Parameters:**
- `sql` (required): INSERT, UPDATE, or DELETE statement
- `dry_run` (optional): If true, shows affected rows without executing (default: true)
- `confirm_token` (optional): Token from dry-run response, required when dry_run=false

**Example - Step 1 (Dry Run):**
```json
{
  "name": "execute",
  "arguments": {
    "sql": "UPDATE users SET status = 'inactive' WHERE last_login < '2024-01-01'",
    "dry_run": true
  }
}
```

**Response:**
```json
{
  "content": [
    {"type": "text", "text": "DRY RUN - Operation: UPDATE"},
    {"type": "text", "text": "This operation will affect 42 rows"},
    {"type": "text", "text": "To execute this query, run again with dry_run=false and the confirmation token below:"},
    {"type": "text", "text": "confirm_token: abc123def456"}
  ],
  "affected_rows": 42,
  "operation": "UPDATE",
  "confirm_token": "abc123def456"
}
```

**Example - Step 2 (Execute):**
```json
{
  "name": "execute",
  "arguments": {
    "sql": "UPDATE users SET status = 'inactive' WHERE last_login < '2024-01-01'",
    "dry_run": false,
    "confirm_token": "abc123def456"
  }
}
```

### schema
Get the schema of a MySQL table.

**Parameters:**
- `table` (required): The name of the table

**Example:**
```json
{
  "name": "schema",
  "arguments": {
    "table": "users"
  }
}
```

### tables
List all tables in the database.

**Example:**
```json
{
  "name": "tables",
  "arguments": {}
}
```

### explain
Analyze the execution plan of a MySQL query to understand performance.

**Parameters:**
- `query` (required): The SQL query to analyze

**Example:**
```json
{
  "name": "explain",
  "arguments": {
    "query": "SELECT * FROM users WHERE email = 'test@example.com'"
  }
}
```

## Integration with AI Tools

### VSCode Integration

#### Option 1: Using MCP Client Extensions

Configure in VSCode settings.json:
```json
{
  "mcp.servers": {
    "mysql": {
      "command": "/path/to/mysql-mcp-server",
      "env": {
        "MYSQL_HOST": "localhost",
        "MYSQL_USER": "root",
        "MYSQL_PASSWORD": "password",
        "MYSQL_DATABASE": "mydb"
      }
    }
  }
}
```

#### Option 2: Custom VSCode Extension

Create a custom extension that spawns the MCP server. See [INTEGRATION.md](docs/INTEGRATION.md) for implementation details.

### Cursor Integration

Cursor supports MCP servers through its configuration:

1. Open Cursor Settings
2. Navigate to "AI" → "Model Context Protocol"
3. Add server configuration:

```json
{
  "mysql": {
    "command": "/path/to/mysql-mcp-server",
    "env": {
      "MYSQL_HOST": "localhost",
      "MYSQL_USER": "root",
      "MYSQL_PASSWORD": "password",
      "MYSQL_DATABASE": "mydb"
    }
  }
}
```

### GitHub Copilot Integration

GitHub Copilot doesn't directly support MCP servers, but you can create a bridge through VSCode extensions. See [INTEGRATION.md](docs/INTEGRATION.md) for detailed implementation.

### Generic Integration Pattern

For any tool that supports subprocess communication:

```javascript
const { spawn } = require('child_process');

class MCPClient {
    constructor(serverPath, env) {
        this.server = spawn(serverPath, [], { env });
        // ... handle communication
    }

    async callTool(name, arguments) {
        return this.request('tools/call', { name, arguments });
    }
}

// Usage
const client = new MCPClient('/path/to/mysql-mcp-server', {
    MYSQL_HOST: 'localhost',
    MYSQL_USER: 'root',
    MYSQL_PASSWORD: 'password',
    MYSQL_DATABASE: 'mydb'
});
```

For complete integration examples and troubleshooting, see [INTEGRATION.md](docs/INTEGRATION.md).

## Testing

For detailed testing instructions, see [TESTING.md](docs/TESTING.md).

Quick start:
```bash
# Setup and run interactive test client
make setup
make test-client
```

## Development

Run tests:
```bash
make test
```

Run the server:
```bash
make run
```

## Security Considerations

- The `query` tool is restricted to SELECT statements only to prevent accidental data modification
- The `execute` tool requires a two-step confirmation process for all data modification operations
- Never expose this server to untrusted clients
- Use appropriate MySQL user permissions
- Consider using read-only database users when possible
- The dry-run feature allows you to preview the impact of UPDATE/DELETE operations before execution
- Confirmation tokens expire after 5 minutes for security
- Keep your database credentials secure

## License

MIT