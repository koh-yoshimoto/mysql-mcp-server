package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/koh-yoshimoto/mysql-mcp-server/cache"
	"github.com/koh-yoshimoto/mysql-mcp-server/format"
	"github.com/koh-yoshimoto/mysql-mcp-server/mysql"
	"github.com/tidwall/gjson"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPServer struct {
	reader        *bufio.Reader
	writer        io.Writer
	mysqlClient   *mysql.Client
	queryCache    *cache.QueryCache
	confirmTokens map[string]*ExecuteConfirmation
}

type ExecuteConfirmation struct {
	SQL          string
	AffectedRows int64
	Operation    string
	Table        string
	CreatedAt    time.Time
}

func NewMCPServer() *MCPServer {
	return &MCPServer{
		reader:        bufio.NewReader(os.Stdin),
		writer:        os.Stdout,
		confirmTokens: make(map[string]*ExecuteConfirmation),
	}
}

func (s *MCPServer) InitMySQL() error {
	config := &mysql.Config{
		Host:     os.Getenv("MYSQL_HOST"),
		Port:     3306,
		User:     os.Getenv("MYSQL_USER"),
		Password: os.Getenv("MYSQL_PASSWORD"),
		Database: os.Getenv("MYSQL_DATABASE"),
	}

	if config.Host == "" {
		config.Host = "localhost"
	}

	if portStr := os.Getenv("MYSQL_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err == nil {
			config.Port = port
		}
	}

	client, err := mysql.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create MySQL client: %w", err)
	}

	s.mysqlClient = client
	return nil
}

func (s *MCPServer) Start() {
	log.SetOutput(os.Stderr)
	log.Println("MySQL MCP Server starting...")

	if err := s.InitMySQL(); err != nil {
		log.Printf("Warning: Could not initialize MySQL: %v", err)
		log.Println("MySQL tools will not be available until connection is established")
	} else {
		log.Println("MySQL connection established")

		// Initialize cache with 5 minute TTL and 1000 max entries
		s.queryCache = cache.NewQueryCache(5*time.Minute, 1000)
		log.Println("Query cache initialized")
	}

	defer func() {
		if s.mysqlClient != nil {
			s.mysqlClient.Close()
		}
	}()

	scanner := bufio.NewScanner(s.reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		response := s.handleRequest(&req)
		if err := s.sendResponse(response); err != nil {
			log.Printf("Error sending response: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

func (s *MCPServer) handleRequest(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *MCPServer) handleInitialize(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "mysql-mcp-server",
				"version": "0.1.0",
			},
		},
	}
}

func (s *MCPServer) handleToolsList(req *Request) *Response {
	tools := []map[string]interface{}{
		{
			"name":        "query",
			"description": "Execute SELECT queries to retrieve data from MySQL database. For INSERT, UPDATE, DELETE operations, use the 'execute' tool instead.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SELECT statement only. Example: SELECT * FROM users WHERE age > 18",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"json", "table", "csv", "markdown"},
						"default":     "table",
						"description": "Output format for results",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "execute",
			"description": "Execute INSERT, UPDATE, DELETE queries with mandatory safety checks. IMPORTANT: Always run with dry_run=true first, show the results to the user, and ask for explicit confirmation before executing with dry_run=false.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "INSERT, UPDATE, or DELETE statement. Example: UPDATE users SET status='active' WHERE id=123",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, previews the operation without executing. ALWAYS use true first and ask user for confirmation before setting to false.",
						"default":     true,
					},
					"confirm_token": map[string]interface{}{
						"type":        "string",
						"description": "Token from dry-run response. Required when dry_run=false. Only use after user explicitly confirms the operation.",
					},
				},
				"required":             []string{"sql"},
				"additionalProperties": false,
			},
		},
		{
			"name":        "schema",
			"description": "Get the schema of a MySQL table",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "The name of the table",
					},
				},
				"required": []string{"table"},
			},
		},
		{
			"name":        "tables",
			"description": "List all tables in the database",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "explain",
			"description": "Analyze the execution plan of a MySQL query to understand performance. Use analyze=true to get actual execution statistics (EXPLAIN ANALYZE).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The SQL query to analyze",
					},
					"analyze": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, runs EXPLAIN ANALYZE to get actual execution statistics. Note: This will execute the query.",
						"default":     false,
					},
				},
				"required": []string{"query"},
			},
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *MCPServer) handleToolsCall(req *Request) *Response {
	if s.mysqlClient == nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32603,
				Message: "MySQL connection not established",
			},
		}
	}

	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	switch params.Name {
	case "query":
		return s.handleQueryTool(req.ID, params.Arguments)
	case "execute":
		return s.handleExecuteTool(req.ID, params.Arguments)
	case "schema":
		return s.handleSchemaTool(req.ID, params.Arguments)
	case "tables":
		return s.handleTablesTool(req.ID)
	case "explain":
		return s.handleExplainTool(req.ID, params.Arguments)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}
}

// isSelectQuery checks if the query is a SELECT statement
func isSelectQuery(query string) bool {
	// Trim whitespace and convert to uppercase for comparison
	trimmed := strings.TrimSpace(strings.ToUpper(query))

	// Check if it starts with SELECT (handle comments and whitespace)
	selectRegex := regexp.MustCompile(`^\s*(--.*\n|/\*.*?\*/)*\s*SELECT`)
	return selectRegex.MatchString(trimmed)
}

// detectQueryOperation detects the SQL operation type
func detectQueryOperation(query string) string {
	trimmed := strings.TrimSpace(strings.ToUpper(query))

	operations := []string{"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "TRUNCATE", "REPLACE"}
	for _, op := range operations {
		if strings.HasPrefix(trimmed, op) {
			return op
		}
	}

	// Handle queries with comments
	operationRegex := regexp.MustCompile(`^\s*(--.*\n|/\*.*?\*/)*\s*(INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|TRUNCATE|REPLACE)`)
	matches := operationRegex.FindStringSubmatch(trimmed)
	if len(matches) > 2 {
		return matches[2]
	}

	return "UNKNOWN"
}

// generateConfirmToken generates a unique token for execute confirmation
func generateConfirmToken(sql string, affectedRows int64) string {
	data := fmt.Sprintf("%s_%d_%d", sql, affectedRows, time.Now().UnixNano())
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])[:12]
}

// convertToInt64 converts various types to int64
func convertToInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case uint64:
		return int64(val), nil
	case uint:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	case float32:
		return int64(val), nil
	case string:
		var result int64
		_, err := fmt.Sscanf(val, "%d", &result)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string '%s' to int64: %w", val, err)
		}
		return result, nil
	case []byte:
		return convertToInt64(string(val))
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func (s *MCPServer) handleQueryTool(id interface{}, args json.RawMessage) *Response {
	query := gjson.GetBytes(args, "query").String()
	if query == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: "Query parameter is required",
			},
		}
	}

	// Validate that this is a SELECT query
	if !isSelectQuery(query) {
		operation := detectQueryOperation(query)
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: fmt.Sprintf("This tool only supports SELECT queries. For %s operations, please use the 'execute' tool instead. Use the 'execute' tool with dry_run=true first to preview changes before executing data modification queries.", operation),
			},
		}
	}

	// Get format preference
	outputFormat := gjson.GetBytes(args, "format").String()
	if outputFormat == "" {
		outputFormat = "table"
	}

	start := time.Now()

	// Check cache first if available
	if s.queryCache != nil {
		if cachedResults, found := s.queryCache.Get(query); found {
			log.Printf("Cache hit for query: %s", query)
			executionTime := time.Since(start)

			formattedOutput := s.formatResults(cachedResults, outputFormat)

			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Result: map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Query executed in %dms (cached). %d rows returned.",
								executionTime.Milliseconds(), len(cachedResults)),
						},
						{
							"type": "text",
							"text": formattedOutput,
						},
					},
				},
			}
		}
	}

	results, err := s.mysqlClient.Query(query)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32603,
				Message: fmt.Sprintf("Query failed: %v", err),
			},
		}
	}

	executionTime := time.Since(start)

	// Cache the results if cache is available
	if s.queryCache != nil {
		s.queryCache.Set(query, results)
	}

	formattedOutput := s.formatResults(results, outputFormat)

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Query executed in %dms. %d rows returned.",
						executionTime.Milliseconds(), len(results)),
				},
				{
					"type": "text",
					"text": formattedOutput,
				},
			},
		},
	}
}

func (s *MCPServer) handleSchemaTool(id interface{}, args json.RawMessage) *Response {
	table := gjson.GetBytes(args, "table").String()
	if table == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: "Table parameter is required",
			},
		}
	}

	schema, err := s.mysqlClient.GetTableSchema(table)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32603,
				Message: fmt.Sprintf("Failed to get schema: %v", err),
			},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Schema for table '%s':", table),
				},
				{
					"type": "text",
					"text": formatResults(schema),
				},
			},
		},
	}
}

func (s *MCPServer) handleTablesTool(id interface{}) *Response {
	tables, err := s.mysqlClient.GetTables()
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32603,
				Message: fmt.Sprintf("Failed to get tables: %v", err),
			},
		}
	}

	tableList := ""
	for _, table := range tables {
		tableList += fmt.Sprintf("- %s\n", table)
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Found %d tables:", len(tables)),
				},
				{
					"type": "text",
					"text": tableList,
				},
			},
		},
	}
}

func (s *MCPServer) handleExplainTool(id interface{}, args json.RawMessage) *Response {
	query := gjson.GetBytes(args, "query").String()
	if query == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: "Missing required parameter: query",
			},
		}
	}

	// Get analyze option
	analyze := gjson.GetBytes(args, "analyze").Bool()

	// Validate that this is a SELECT query when using EXPLAIN ANALYZE
	if analyze && !isSelectQuery(query) {
		operation := detectQueryOperation(query)

		// Provide helpful suggestion based on operation type
		suggestion := ""
		switch operation {
		case "UPDATE":
			// Extract table name from UPDATE query
			re := regexp.MustCompile(`(?i)UPDATE\s+(\S+)\s+SET`)
			matches := re.FindStringSubmatch(query)
			if len(matches) > 1 {
				table := matches[1]
				suggestion = fmt.Sprintf("To analyze UPDATE performance, try: SELECT * FROM %s WHERE <your conditions>", table)
			}
		case "DELETE":
			// Extract table name from DELETE query
			re := regexp.MustCompile(`(?i)DELETE\s+FROM\s+(\S+)`)
			matches := re.FindStringSubmatch(query)
			if len(matches) > 1 {
				table := matches[1]
				suggestion = fmt.Sprintf("To analyze DELETE performance, try: SELECT * FROM %s WHERE <your conditions>", table)
			}
		case "INSERT":
			suggestion = "To analyze INSERT performance, examine the table structure with 'schema <table>' or analyze a SELECT on the target table"
		default:
			suggestion = "EXPLAIN ANALYZE can only be used with SELECT queries as it executes the query"
		}

		errorMessage := fmt.Sprintf("EXPLAIN ANALYZE cannot be used with %s queries as it would execute the modification. ", operation)
		errorMessage += "EXPLAIN ANALYZE actually executes the query, so for safety it's restricted to SELECT queries only. "
		errorMessage += suggestion + ". "
		errorMessage += "Alternative: Use EXPLAIN (without ANALYZE) to see the execution plan without running the query."
		
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: errorMessage,
			},
		}
	}

	// Prepare EXPLAIN query
	explainPrefix := "EXPLAIN"
	if analyze {
		explainPrefix = "EXPLAIN ANALYZE"
	}
	explainQuery := explainPrefix + " " + query

	// Execute the EXPLAIN query
	results, err := s.mysqlClient.Query(explainQuery)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32603,
				Message: fmt.Sprintf("Failed to explain query: %v", err),
			},
		}
	}

	// Return raw EXPLAIN results in table format
	formattedOutput := s.formatResults(results, "table")

	// Prepare header text
	headerText := fmt.Sprintf("Execution plan for: %s", query)
	if analyze {
		headerText = fmt.Sprintf("Execution plan with actual statistics for: %s", query)
	}

	contentMessages := []map[string]interface{}{
		{
			"type": "text",
			"text": headerText,
		},
	}

	if analyze {
		contentMessages = append(contentMessages, map[string]interface{}{
			"type": "text",
			"text": "⚠️  Note: EXPLAIN ANALYZE actually executes the query to gather statistics.",
		})
	}

	contentMessages = append(contentMessages, map[string]interface{}{
		"type": "text",
		"text": formattedOutput,
	})

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": contentMessages,
		},
	}
}

func (s *MCPServer) handleExecuteTool(id interface{}, args json.RawMessage) *Response {
	sql := gjson.GetBytes(args, "sql").String()
	if sql == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: "SQL parameter is required",
			},
		}
	}

	// Check if this is a SELECT query - redirect to query tool
	if isSelectQuery(sql) {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32602,
				Message: "SELECT queries should use the 'query' tool instead. Use the 'query' tool for SELECT statements.",
			},
		}
	}

	dryRun := gjson.GetBytes(args, "dry_run").Bool()
	confirmToken := gjson.GetBytes(args, "confirm_token").String()

	// Default to dry_run=true if not specified
	if !gjson.GetBytes(args, "dry_run").Exists() {
		dryRun = true
	}

	// If not dry run, check for valid confirmation token
	if !dryRun {
		if confirmToken == "" {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error: &Error{
					Code:    -32602,
					Message: "confirm_token is required when dry_run=false",
				},
			}
		}

		// Validate token
		confirmation, exists := s.confirmTokens[confirmToken]
		if !exists {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error: &Error{
					Code:    -32602,
					Message: "Invalid or expired confirmation token",
				},
			}
		}

		// Check if token is expired (5 minutes)
		if time.Since(confirmation.CreatedAt) > 5*time.Minute {
			delete(s.confirmTokens, confirmToken)
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error: &Error{
					Code:    -32602,
					Message: "Confirmation token has expired. Please run with dry_run=true again.",
				},
			}
		}

		// Verify SQL matches
		if confirmation.SQL != sql {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error: &Error{
					Code:    -32602,
					Message: "SQL does not match the confirmation token",
				},
			}
		}

		// Execute the query
		result, err := s.mysqlClient.Execute(sql)
		if err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error: &Error{
					Code:    -32603,
					Message: fmt.Sprintf("Execution failed: %v", err),
				},
			}
		}

		// Clean up token
		delete(s.confirmTokens, confirmToken)

		rowsAffected, _ := result.RowsAffected()

		// Prepare execution summary
		executionSummary := fmt.Sprintf("✅ %s operation completed successfully", confirmation.Operation)
		affectedSummary := fmt.Sprintf("📊 Rows affected: %d", rowsAffected)

		// Check if actual affected rows match the estimate
		rowDifference := ""
		if confirmation.AffectedRows > 0 && rowsAffected != confirmation.AffectedRows {
			diff := rowsAffected - confirmation.AffectedRows
			if diff > 0 {
				rowDifference = fmt.Sprintf("ℹ️  Note: Actual affected rows (%d) exceeded estimate (%d) by %d rows",
					rowsAffected, confirmation.AffectedRows, diff)
			} else {
				rowDifference = fmt.Sprintf("ℹ️  Note: Actual affected rows (%d) were less than estimate (%d)",
					rowsAffected, confirmation.AffectedRows)
			}
		}

		contentMessages := []map[string]interface{}{
			{
				"type": "text",
				"text": executionSummary,
			},
			{
				"type": "text",
				"text": affectedSummary,
			},
		}

		if rowDifference != "" {
			contentMessages = append(contentMessages, map[string]interface{}{
				"type": "text",
				"text": rowDifference,
			})
		}

		contentMessages = append(contentMessages, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("🔍 SQL executed: %s", sql),
		})

		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Result: map[string]interface{}{
				"content":        contentMessages,
				"success":        true,
				"operation":      confirmation.Operation,
				"rows_affected":  rowsAffected,
				"estimated_rows": confirmation.AffectedRows,
			},
		}
	}

	// Dry run mode - analyze the query
	operation := detectQueryOperation(sql)

	// For dry run, we need to estimate affected rows
	isExactCount := false
	affectedRows, err := s.estimateAffectedRows(sql)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &Error{
				Code:    -32603,
				Message: fmt.Sprintf("Failed to analyze query: %v", err),
			},
		}
	}

	// Check if we can use transaction method
	if s.mysqlClient.CanUseTransaction(sql) {
		isExactCount = true
	}

	// Generate confirmation token
	token := generateConfirmToken(sql, affectedRows)

	// Store confirmation
	s.confirmTokens[token] = &ExecuteConfirmation{
		SQL:          sql,
		AffectedRows: affectedRows,
		Operation:    operation,
		CreatedAt:    time.Now(),
	}

	// Clean up old tokens
	s.cleanupExpiredTokens()

	warning := ""
	warningDetail := ""
	if affectedRows > 1000 {
		warning = fmt.Sprintf("⚠️  WARNING: This operation will affect %d rows", affectedRows)
		warningDetail = "This is a large number of rows. Please ensure this is intentional."
	} else if affectedRows == -1 {
		warning = "⚠️  WARNING: Unable to estimate affected rows for this operation"
		warningDetail = fmt.Sprintf("Operation type: %s - This operation may affect the entire table or database structure.", operation)
	}

	// Add extra warnings for dangerous operations
	isDangerous := false
	if operation == "DROP" || operation == "TRUNCATE" || operation == "ALTER" {
		isDangerous = true
		if operation == "DROP" {
			warning = "🚨 CRITICAL: DROP operation will permanently delete database objects"
			warningDetail = "This operation cannot be undone. All data will be permanently lost."
		} else if operation == "TRUNCATE" {
			warning = "🚨 CRITICAL: TRUNCATE will delete ALL rows from the table"
			warningDetail = "This operation is faster than DELETE but cannot be rolled back and resets AUTO_INCREMENT."
		}
	}

	// Prepare AI instruction
	confirmationQuestion := fmt.Sprintf("Do you want to proceed with this %s operation that will affect %d rows?", operation, affectedRows)
	if isDangerous {
		confirmationQuestion = fmt.Sprintf("⚠️  This is a DANGEROUS %s operation. Are you ABSOLUTELY SURE you want to proceed? Please type 'yes' to confirm.", operation)
	}

	aiInstruction := fmt.Sprintf(`IMPORTANT: Before executing this query, you MUST:
1. Show the user this dry-run result: %s operation will affect %d rows
2. Ask the user explicitly: "%s"
3. Only proceed with execution if the user clearly confirms (yes, proceed, confirm, etc.)
4. If the user declines or is unsure, do not execute the query`, operation, affectedRows, confirmationQuestion)

	if warning != "" {
		aiInstruction += fmt.Sprintf("\n5. Emphasize the warning: %s", warning)
	}

	if isDangerous {
		aiInstruction += "\n6. For dangerous operations (DROP, TRUNCATE), require explicit 'yes' confirmation"
		aiInstruction += "\n7. Remind the user this operation cannot be undone"
	}

	affectedRowsText := fmt.Sprintf("📊 Affected rows: %d", affectedRows)
	if isExactCount {
		affectedRowsText = fmt.Sprintf("📊 Affected rows: %d (exact count using transaction rollback)", affectedRows)
	} else if affectedRows == -1 {
		affectedRowsText = "📊 Affected rows: Cannot be determined (DDL statement)"
	}

	contentMessages := []map[string]interface{}{
		{
			"type": "text",
			"text": fmt.Sprintf("🔍 DRY RUN RESULT - Operation: %s", operation),
		},
		{
			"type": "text",
			"text": affectedRowsText,
		},
	}

	if warning != "" {
		contentMessages = append(contentMessages, map[string]interface{}{
			"type": "text",
			"text": warning,
		})
		if warningDetail != "" {
			contentMessages = append(contentMessages, map[string]interface{}{
				"type": "text",
				"text": warningDetail,
			})
		}
	}

	contentMessages = append(contentMessages,
		map[string]interface{}{
			"type": "text",
			"text": "💡 To execute this query after user confirmation:",
		},
		map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("Use the execute tool with dry_run=false and confirm_token='%s'", token),
		},
	)

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content":                    contentMessages,
			"affected_rows":              affectedRows,
			"operation":                  operation,
			"confirm_token":              token,
			"warning":                    warning,
			"ai_instruction":             aiInstruction,
			"requires_user_confirmation": true,
			"confirmation_prompt":        confirmationQuestion,
			"is_dangerous_operation":     isDangerous,
			"is_exact_count":             isExactCount,
		},
	}
}

func (s *MCPServer) estimateAffectedRows(sql string) (int64, error) {
	// First, try to use transaction method for accurate results
	if s.mysqlClient.CanUseTransaction(sql) {
		affectedRows, err := s.mysqlClient.ExecuteInTransaction(sql)
		if err == nil {
			// Successfully got exact count using transaction
			return affectedRows, nil
		}
		// If transaction method failed, fall back to estimation
		log.Printf("Transaction method failed, falling back to estimation: %v", err)
	}

	// Fall back to estimation for DDL statements or if transaction failed
	operation := detectQueryOperation(sql)

	switch operation {
	case "DELETE":
		// Convert DELETE to SELECT COUNT(*) to estimate rows
		selectQuery := regexp.MustCompile(`(?i)DELETE\s+FROM`).ReplaceAllString(sql, "SELECT COUNT(*) as count FROM")
		results, err := s.mysqlClient.Query(selectQuery)
		if err != nil {
			return 0, err
		}
		if len(results) > 0 {
			if countVal, exists := results[0]["count"]; exists {
				count, err := convertToInt64(countVal)
				if err != nil {
					return 0, fmt.Errorf("failed to convert count: %w", err)
				}
				return count, nil
			}
		}

	case "UPDATE":
		// Convert UPDATE to SELECT COUNT(*) to estimate rows
		// This is simplified - proper parsing would be better
		re := regexp.MustCompile(`(?i)UPDATE\s+(\S+)\s+SET\s+.*?(WHERE.*)?$`)
		matches := re.FindStringSubmatch(sql)
		if len(matches) > 1 {
			table := matches[1]
			whereClause := ""
			if len(matches) > 2 && matches[2] != "" {
				whereClause = matches[2]
			}
			selectQuery := fmt.Sprintf("SELECT COUNT(*) as count FROM %s %s", table, whereClause)
			results, err := s.mysqlClient.Query(selectQuery)
			if err != nil {
				return 0, err
			}
			if len(results) > 0 {
				switch v := results[0]["count"].(type) {
				case int64:
					return v, nil
				case string:
					var count int64
					fmt.Sscanf(v, "%d", &count)
					return count, nil
				}
			}
		}

	case "INSERT":
		// For INSERT, try to count VALUES clauses for bulk inserts
		// This is a simple approach - counting commas between VALUES and end/ON
		if strings.Contains(strings.ToUpper(sql), "VALUES") {
			valuesIndex := strings.Index(strings.ToUpper(sql), "VALUES")
			if valuesIndex != -1 {
				valuesPart := sql[valuesIndex+6:]
				// Count opening parentheses as a rough estimate of rows
				rowCount := int64(strings.Count(valuesPart, "("))
				if rowCount > 0 {
					return rowCount, nil
				}
			}
		}
		return 1, nil

	case "TRUNCATE":
		// For TRUNCATE, get the total count of rows in the table
		re := regexp.MustCompile(`(?i)TRUNCATE\s+(?:TABLE\s+)?(\S+)`)
		matches := re.FindStringSubmatch(sql)
		if len(matches) > 1 {
			table := strings.Trim(matches[1], "`\"'")
			selectQuery := fmt.Sprintf("SELECT COUNT(*) as count FROM `%s`", table)
			results, err := s.mysqlClient.Query(selectQuery)
			if err != nil {
				// If we can't get count, return -1 to indicate unknown
				return -1, nil
			}
			if len(results) > 0 {
				if countVal, exists := results[0]["count"]; exists {
					count, err := convertToInt64(countVal)
					if err != nil {
						return -1, nil
					}
					return count, nil
				}
			}
		}
		return -1, nil

	case "DROP":
		// For DROP operations, we can't estimate without more complex parsing
		// Would need to check if it's DROP TABLE, DROP DATABASE, etc.
		return -1, nil
	}

	return 0, nil
}

func (s *MCPServer) cleanupExpiredTokens() {
	now := time.Now()
	for token, confirmation := range s.confirmTokens {
		if now.Sub(confirmation.CreatedAt) > 5*time.Minute {
			delete(s.confirmTokens, token)
		}
	}
}

func (s *MCPServer) formatResults(results []map[string]interface{}, outputFormat string) string {
	if len(results) == 0 {
		return "No results"
	}

	switch outputFormat {
	case "table":
		formatter := format.NewTableFormatter(results)
		return formatter.Render()
	case "csv":
		return format.FormatCSV(results)
	case "markdown":
		return format.FormatMarkdown(results)
	case "json":
		fallthrough
	default:
		output, _ := json.MarshalIndent(results, "", "  ")
		return string(output)
	}
}

func formatResults(results []map[string]interface{}) string {
	if len(results) == 0 {
		return "No results"
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return string(output)
}

func (s *MCPServer) sendResponse(resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	return err
}

func (s *MCPServer) sendError(id interface{}, code int, message string) error {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	return s.sendResponse(resp)
}

var Version = "0.1.3"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("mysql-mcp-server %s\n", Version)
		os.Exit(0)
	}

	server := NewMCPServer()
	server.Start()
}
