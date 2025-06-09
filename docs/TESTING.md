# Testing Guide

This guide covers testing procedures for the MySQL MCP Server.

## Quick Start with Docker

The project includes a Docker setup for easy testing:

```bash
# Start MySQL test instance
make docker-up

# Test the server interactively
make test-client

# Or start the server manually
make test-server
```

## Manual Testing Steps

1. **Setup and run tests:**
   ```bash
   # Quick setup with Docker
   make setup
   
   # Run interactive test client
   make test-client
   
   # Or run the server manually
   make test-server
   ```

## Sample Test Queries

The test database includes sample data. Try these queries:

```sql
-- List all users
SELECT * FROM users;

-- Show products by category
SELECT * FROM products WHERE category = 'Electronics';

-- Order summary view
SELECT * FROM order_summary;

-- User order statistics
SELECT 
    u.username, 
    COUNT(o.id) as order_count, 
    SUM(o.total_amount) as total_spent 
FROM users u 
LEFT JOIN orders o ON u.id = o.user_id 
GROUP BY u.id;

-- Call stored procedure
CALL GetUserOrders(1);
```

## Test Different Output Formats

Test the formatting options:

```json
// Table format (default)
{
  "name": "query",
  "arguments": {
    "query": "SELECT * FROM users LIMIT 3",
    "format": "table"
  }
}

// CSV format
{
  "name": "query", 
  "arguments": {
    "query": "SELECT * FROM products LIMIT 5",
    "format": "csv"
  }
}

// Markdown format
{
  "name": "query",
  "arguments": {
    "query": "SELECT * FROM order_summary",
    "format": "markdown" 
  }
}
```

## Database Management

- **PhpMyAdmin** (optional): `make phpmyadmin` then visit http://localhost:8080
- **Reset database**: `make docker-reset`
- **View logs**: `make docker-logs`
- **Clean up**: `make docker-down`

## Available Make Commands

```bash
make setup          # Initial setup
make docker-up       # Start MySQL container
make docker-down     # Stop containers
make docker-reset    # Reset database with fresh data
make test-server     # Run MCP server with test DB
make test-client     # Run interactive test client
make demo-queries    # Show sample queries
make clean          # Clean up everything
```

## Unit Tests

Run Go unit tests:

```bash
make test
```

## Interactive Test Client

The interactive test client (`test/test-client.js`) provides an easy way to test the MCP server:

1. It automatically starts the server with test database credentials
2. Runs a series of automated tests
3. Enters interactive mode where you can:
   - Type SQL queries directly
   - Use `tables` to list all tables
   - Use `schema <table>` to view table structure
   - Type `exit` to quit

## Docker Test Environment

The test environment uses Docker Compose to provide:

- MySQL 8.0 server with test data
- Pre-configured database schema
- Sample data for testing queries
- Optional PhpMyAdmin for database inspection

Test credentials:
- Host: localhost
- Port: 3306
- User: testuser
- Password: testpass
- Database: testdb