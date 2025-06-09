# MySQL MCP Server

A Model Context Protocol (MCP) server that provides MySQL database access through standardized tools.

## Features

- **Query Execution**: Execute arbitrary SQL queries
- **Schema Inspection**: View table schemas
- **Table Listing**: List all tables in the database

## Requirements

- Go 1.21+ (developed with Go 1.23.6)
- MySQL 5.7+ or MySQL 8.0+
- Make (for build commands)

## Installation

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
      "command": "/path/to/mysql-mcp-server",
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
Execute a MySQL query.

**Parameters:**
- `query` (required): The SQL query to execute

**Example:**
```json
{
  "name": "query",
  "arguments": {
    "query": "SELECT * FROM users LIMIT 10"
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

- Never expose this server to untrusted clients as it allows arbitrary SQL execution
- Use appropriate MySQL user permissions
- Consider using read-only database users when possible
- Keep your database credentials secure

## License

MIT