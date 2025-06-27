#!/usr/bin/env node

const { spawn } = require('child_process');

class ExplainAnalyzeTest {
    constructor() {
        this.server = null;
        this.requestId = 0;
        this.pending = new Map();
    }

    start() {
        console.log('Starting MySQL MCP Server for EXPLAIN ANALYZE test...');
        
        this.server = spawn('./mysql-mcp-server', [], {
            env: {
                ...process.env,
                MYSQL_HOST: 'localhost',
                MYSQL_PORT: '3306',
                MYSQL_USER: 'root',
                MYSQL_PASSWORD: '',
                MYSQL_DATABASE: 'test'
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
                    // Ignore non-JSON lines
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

    async runTest() {
        // Wait for server to start
        await new Promise(resolve => setTimeout(resolve, 1000));

        console.log('\n=== Testing EXPLAIN ANALYZE with UPDATE query ===\n');

        // Initialize
        await this.request('initialize', {
            protocolVersion: "2024-11-05",
            capabilities: {}
        });

        // Test EXPLAIN ANALYZE with UPDATE
        console.log('\nTesting EXPLAIN ANALYZE with UPDATE query...');
        const explainRequest = {
            query: "UPDATE users SET last_login = NOW() WHERE id = 1",
            analyze: true
        };
        
        console.log('Request arguments:', JSON.stringify(explainRequest, null, 2));
        
        const response = await this.request('tools/call', {
            name: 'explain',
            arguments: explainRequest
        });
        
        console.log('← Response:', JSON.stringify(response, null, 2));
        
        // Validate response format
        if (response.error && response.result) {
            console.error('✗ INVALID RESPONSE: Both error and result fields present!');
            console.error('This violates MCP protocol specification');
            process.exit(1);
        } else if (response.error) {
            console.log('✓ Valid error response (no result field)');
            console.log('Error message:', response.error.message);
        } else {
            console.log('✗ Unexpected: No error for UPDATE with EXPLAIN ANALYZE');
        }

        console.log('\n=== Test completed successfully ===');
        this.server.kill();
        process.exit(0);
    }
}

// Run test
const tester = new ExplainAnalyzeTest();
tester.start();
tester.runTest().catch(err => {
    console.error('Test failed:', err);
    process.exit(1);
});