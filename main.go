package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
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
	reader      *bufio.Reader
	writer      io.Writer
	mysqlClient *mysql.Client
	queryCache  *cache.QueryCache
}

func NewMCPServer() *MCPServer {
	return &MCPServer{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
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
			"description": "Execute a MySQL query",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The SQL query to execute",
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
	case "schema":
		return s.handleSchemaTool(req.ID, params.Arguments)
	case "tables":
		return s.handleTablesTool(req.ID)
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

var Version = "0.1.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("mysql-mcp-server %s\n", Version)
		os.Exit(0)
	}

	server := NewMCPServer()
	server.Start()
}
