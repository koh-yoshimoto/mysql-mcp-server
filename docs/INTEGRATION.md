# Integration Guide

This guide explains how to integrate the MySQL MCP Server with various AI development tools.

## VSCode Integration

### Option 1: Using MCP Client Extensions

Several VSCode extensions support MCP servers:

1. **MCP Client for VS Code** (if available)
   - Install the extension from VSCode marketplace
   - Configure in settings.json:
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

### Option 2: Custom VSCode Extension

Create a custom extension that spawns the MCP server:

```typescript
import * as vscode from 'vscode';
import { spawn } from 'child_process';

export function activate(context: vscode.ExtensionContext) {
    const mcpServer = spawn('/path/to/mysql-mcp-server', [], {
        env: {
            ...process.env,
            MYSQL_HOST: 'localhost',
            MYSQL_USER: 'root',
            MYSQL_PASSWORD: 'password',
            MYSQL_DATABASE: 'mydb'
        }
    });

    // Handle stdio communication
    mcpServer.stdout.on('data', (data) => {
        // Parse MCP responses
        const response = JSON.parse(data.toString());
        // Handle response
    });

    // Send MCP requests
    function sendRequest(request: any) {
        mcpServer.stdin.write(JSON.stringify(request) + '\n');
    }
}
```

## Cursor Integration

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

## GitHub Copilot Integration

GitHub Copilot doesn't directly support MCP servers, but you can create a bridge:

### Option 1: Copilot Chat Extension

Create a VSCode extension that provides MySQL context to Copilot:

```typescript
vscode.chat.registerChatParticipant('mysql-assistant', async (request, context, response, token) => {
    // Spawn MCP server
    const mcpServer = spawn('/path/to/mysql-mcp-server');
    
    // Forward requests to MCP server
    mcpServer.stdin.write(JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        method: "tools/call",
        params: {
            name: "query",
            arguments: { query: request.prompt }
        }
    }) + '\n');

    // Return results to Copilot
    mcpServer.stdout.once('data', (data) => {
        const result = JSON.parse(data.toString());
        response.markdown(result.result.content[1].text);
    });
});
```

### Option 2: Custom Language Server

Create a Language Server Protocol (LSP) wrapper around the MCP server to provide database-aware completions.

## Generic Integration Pattern

For any tool that supports subprocess communication:

```javascript
const { spawn } = require('child_process');

class MCPClient {
    constructor(serverPath, env) {
        this.server = spawn(serverPath, [], { env });
        this.requestId = 0;
        this.pending = new Map();
        
        this.server.stdout.on('data', (data) => {
            const lines = data.toString().split('\n').filter(l => l);
            lines.forEach(line => {
                try {
                    const response = JSON.parse(line);
                    const callback = this.pending.get(response.id);
                    if (callback) {
                        callback(response);
                        this.pending.delete(response.id);
                    }
                } catch (e) {
                    console.error('Failed to parse response:', e);
                }
            });
        });
    }

    async initialize() {
        return this.request('initialize', {
            protocolVersion: "2024-11-05",
            capabilities: {}
        });
    }

    async listTools() {
        return this.request('tools/list', {});
    }

    async callTool(name, arguments) {
        return this.request('tools/call', { name, arguments });
    }

    request(method, params) {
        return new Promise((resolve) => {
            const id = ++this.requestId;
            this.pending.set(id, resolve);
            this.server.stdin.write(JSON.stringify({
                jsonrpc: "2.0",
                id,
                method,
                params
            }) + '\n');
        });
    }
}

// Usage
const client = new MCPClient('/path/to/mysql-mcp-server', {
    MYSQL_HOST: 'localhost',
    MYSQL_USER: 'root',
    MYSQL_PASSWORD: 'password',
    MYSQL_DATABASE: 'mydb'
});

await client.initialize();
const tools = await client.listTools();
const result = await client.callTool('query', { query: 'SELECT * FROM users LIMIT 5' });
```

## Testing Integration

Use the provided test client to verify your integration:

```bash
node test-client.js
```

This will:
1. Initialize the MCP connection
2. List available tools
3. Execute sample queries
4. Display results

## Troubleshooting

1. **Connection Issues**
   - Verify MySQL credentials in environment variables
   - Check MySQL server is running and accessible
   - Review server logs (written to stderr)

2. **Communication Issues**
   - Ensure proper line-delimited JSON format
   - Check for proper stdin/stdout handling
   - Verify JSON-RPC request structure

3. **Tool Execution Errors**
   - Validate tool parameters match schema
   - Check MySQL user permissions
   - Review error responses for details