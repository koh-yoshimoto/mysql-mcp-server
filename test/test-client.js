#!/usr/bin/env node

const { spawn } = require('child_process');
const readline = require('readline');

class MCPTestClient {
    constructor() {
        this.server = null;
        this.requestId = 0;
        this.pending = new Map();
    }

    start() {
        console.log('Starting MySQL MCP Server...');
        
        this.server = spawn('./mysql-mcp-server', [], {
            env: {
                ...process.env,
                MYSQL_HOST: process.env.MYSQL_HOST || 'localhost',
                MYSQL_PORT: process.env.MYSQL_PORT || '3306',
                MYSQL_USER: process.env.MYSQL_USER || 'root',
                MYSQL_PASSWORD: process.env.MYSQL_PASSWORD || '',
                MYSQL_DATABASE: process.env.MYSQL_DATABASE || 'test'
            }
        });

        this.server.stdout.on('data', (data) => {
            const lines = data.toString().split('\n').filter(l => l.trim());
            lines.forEach(line => {
                try {
                    const response = JSON.parse(line);
                    const callback = this.pending.get(response.id);
                    if (callback) {
                        callback(response);
                        this.pending.delete(response.id);
                    }
                } catch (e) {
                    console.error('Failed to parse response:', line);
                }
            });
        });

        this.server.stderr.on('data', (data) => {
            console.error('Server log:', data.toString());
        });

        this.server.on('error', (err) => {
            console.error('Failed to start server:', err);
            process.exit(1);
        });

        this.server.on('close', (code) => {
            console.log(`Server exited with code ${code}`);
            process.exit(code);
        });
    }

    request(method, params = {}) {
        return new Promise((resolve) => {
            const id = ++this.requestId;
            this.pending.set(id, resolve);
            
            const request = {
                jsonrpc: "2.0",
                id,
                method,
                params
            };
            
            console.log('\n→ Request:', JSON.stringify(request, null, 2));
            this.server.stdin.write(JSON.stringify(request) + '\n');
        });
    }

    async runTests() {
        console.log('\n=== Running MCP Server Tests ===\n');

        // Test 1: Initialize
        console.log('1. Testing initialize...');
        const initResponse = await this.request('initialize', {
            protocolVersion: "2024-11-05",
            capabilities: {}
        });
        console.log('← Response:', JSON.stringify(initResponse, null, 2));

        // Test 2: List tools
        console.log('\n2. Testing tools/list...');
        const toolsResponse = await this.request('tools/list');
        console.log('← Response:', JSON.stringify(toolsResponse, null, 2));

        // Test 3: List tables
        console.log('\n3. Testing tables tool...');
        const tablesResponse = await this.request('tools/call', {
            name: 'tables',
            arguments: {}
        });
        console.log('← Response:', JSON.stringify(tablesResponse, null, 2));

        // Test 4: Execute query
        console.log('\n4. Testing query tool...');
        const queryResponse = await this.request('tools/call', {
            name: 'query',
            arguments: {
                query: 'SELECT 1 as test, NOW() as current_time'
            }
        });
        console.log('← Response:', JSON.stringify(queryResponse, null, 2));

        // Test 5: Get schema (if tables exist)
        if (tablesResponse.result && tablesResponse.result.content) {
            const tablesText = tablesResponse.result.content[1]?.text || '';
            const firstTable = tablesText.split('\n').find(line => line.startsWith('- '))?.slice(2).trim();
            
            if (firstTable) {
                console.log(`\n5. Testing schema tool for table '${firstTable}'...`);
                const schemaResponse = await this.request('tools/call', {
                    name: 'schema',
                    arguments: {
                        table: firstTable
                    }
                });
                console.log('← Response:', JSON.stringify(schemaResponse, null, 2));
            }
        }

        // Interactive mode
        console.log('\n=== Entering Interactive Mode ===');
        console.log('Type SQL queries or commands:');
        console.log('  - "tables" to list tables');
        console.log('  - "schema <table>" to show table schema');
        console.log('  - Any SQL query');
        console.log('  - "exit" to quit\n');

        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout,
            prompt: 'mysql> '
        });

        rl.prompt();

        rl.on('line', async (line) => {
            const input = line.trim();
            
            if (input.toLowerCase() === 'exit') {
                rl.close();
                this.server.kill();
                return;
            }

            if (input.toLowerCase() === 'tables') {
                const response = await this.request('tools/call', {
                    name: 'tables',
                    arguments: {}
                });
                if (response.result) {
                    console.log(response.result.content.map(c => c.text).join('\n'));
                } else if (response.error) {
                    console.error('Error:', response.error.message);
                }
            } else if (input.toLowerCase().startsWith('schema ')) {
                const table = input.slice(7).trim();
                const response = await this.request('tools/call', {
                    name: 'schema',
                    arguments: { table }
                });
                if (response.result) {
                    console.log(response.result.content.map(c => c.text).join('\n'));
                } else if (response.error) {
                    console.error('Error:', response.error.message);
                }
            } else if (input) {
                const response = await this.request('tools/call', {
                    name: 'query',
                    arguments: { query: input }
                });
                if (response.result) {
                    console.log(response.result.content.map(c => c.text).join('\n'));
                } else if (response.error) {
                    console.error('Error:', response.error.message);
                }
            }

            rl.prompt();
        });

        rl.on('close', () => {
            console.log('\nGoodbye!');
            process.exit(0);
        });
    }
}

// Check if server binary exists
const fs = require('fs');
if (!fs.existsSync('./mysql-mcp-server')) {
    console.error('Error: mysql-mcp-server binary not found.');
    console.error('Please build it first with: go build -o mysql-mcp-server');
    process.exit(1);
}

// Run tests
const client = new MCPTestClient();
client.start();

// Wait a bit for server to start
setTimeout(() => {
    client.runTests().catch(err => {
        console.error('Test failed:', err);
        process.exit(1);
    });
}, 1000);