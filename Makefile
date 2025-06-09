.PHONY: build test run docker-up docker-down docker-reset clean test-server test-client

# Build the MCP server
build:
	go build -o mysql-mcp-server

# Run tests
test:
	go test ./...

# Run the server (for development)
run: build
	./mysql-mcp-server

# Docker commands
docker-up:
	docker compose -f test/docker-compose.yml up -d
	@echo "Waiting for MySQL to be ready..."
	@sleep 5
	@docker compose -f test/docker-compose.yml exec mysql mysqladmin ping -h localhost --silent || (echo "MySQL is not ready yet, waiting..." && sleep 10)
	@echo "MySQL is ready!"

docker-down:
	docker compose -f test/docker-compose.yml down

docker-reset:
	docker compose -f test/docker-compose.yml down -v
	docker compose -f test/docker-compose.yml up -d
	@echo "Waiting for MySQL to be ready..."
	@sleep 5
	@docker compose -f test/docker-compose.yml exec mysql mysqladmin ping -h localhost --silent || (echo "MySQL is not ready yet, waiting..." && sleep 10)
	@echo "MySQL is ready with fresh data!"

# Run with Docker
docker-logs:
	docker compose -f test/docker-compose.yml logs -f mysql

# PhpMyAdmin (for debugging)
phpmyadmin:
	docker compose -f test/docker-compose.yml --profile debug up -d phpmyadmin
	@echo "PhpMyAdmin is available at http://localhost:8080"

# Test the server with Docker MySQL
test-server: build docker-up
	MYSQL_HOST=localhost \
	MYSQL_PORT=3306 \
	MYSQL_USER=testuser \
	MYSQL_PASSWORD=testpass \
	MYSQL_DATABASE=testdb \
	./mysql-mcp-server

# Test with the Node.js client
test-client: build docker-up
	MYSQL_HOST=localhost \
	MYSQL_PORT=3306 \
	MYSQL_USER=testuser \
	MYSQL_PASSWORD=testpass \
	MYSQL_DATABASE=testdb \
	./test/test-client.js

# Full test suite
test-all: docker-reset test-client

# Clean up
clean:
	rm -f mysql-mcp-server
	docker compose -f test/docker-compose.yml down -v

# Show sample queries
demo-queries:
	@echo "Sample queries to test:"
	@echo "1. SELECT * FROM users;"
	@echo "2. SELECT * FROM products WHERE category = 'Electronics';"
	@echo "3. SELECT * FROM order_summary;"
	@echo "4. SELECT u.username, COUNT(o.id) as order_count, SUM(o.total_amount) as total_spent FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id;"
	@echo "5. CALL GetUserOrders(1);"

# Install dependencies
deps:
	go mod download
	go mod tidy

# Development setup
setup: deps docker-up
	@echo "Development environment is ready!"
	@echo "Run 'make test-server' to start the MCP server"
	@echo "Run 'make test-client' to test with the interactive client"
