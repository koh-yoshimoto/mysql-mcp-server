# Output Improvements Guide

This document outlines various improvements to enhance the output formatting and usability of query results.

## 1. Table Formatting

### ASCII Table Format
```go
// format/table.go
package format

import (
	"fmt"
	"strings"
)

type TableFormatter struct {
	headers []string
	rows    [][]string
	widths  []int
}

func NewTableFormatter(results []map[string]interface{}) *TableFormatter {
	if len(results) == 0 {
		return &TableFormatter{}
	}
	
	// Extract headers
	headers := make([]string, 0)
	for key := range results[0] {
		headers = append(headers, key)
	}
	
	// Convert data to strings and calculate widths
	formatter := &TableFormatter{
		headers: headers,
		widths:  make([]int, len(headers)),
	}
	
	// Initialize widths with header lengths
	for i, header := range headers {
		formatter.widths[i] = len(header)
	}
	
	// Process rows
	for _, row := range results {
		rowData := make([]string, len(headers))
		for i, header := range headers {
			value := formatValue(row[header])
			rowData[i] = value
			if len(value) > formatter.widths[i] {
				formatter.widths[i] = len(value)
			}
		}
		formatter.rows = append(formatter.rows, rowData)
	}
	
	return formatter
}

func (f *TableFormatter) Render() string {
	if len(f.headers) == 0 {
		return "No results"
	}
	
	var output strings.Builder
	
	// Top border
	output.WriteString(f.renderBorder("┌", "┬", "┐"))
	
	// Headers
	output.WriteString("│")
	for i, header := range f.headers {
		output.WriteString(fmt.Sprintf(" %-*s │", f.widths[i], header))
	}
	output.WriteString("\n")
	
	// Header separator
	output.WriteString(f.renderBorder("├", "┼", "┤"))
	
	// Rows
	for _, row := range f.rows {
		output.WriteString("│")
		for i, value := range row {
			output.WriteString(fmt.Sprintf(" %-*s │", f.widths[i], value))
		}
		output.WriteString("\n")
	}
	
	// Bottom border
	output.WriteString(f.renderBorder("└", "┴", "┘"))
	
	return output.String()
}

func (f *TableFormatter) renderBorder(left, mid, right string) string {
	var parts []string
	for _, width := range f.widths {
		parts = append(parts, strings.Repeat("─", width+2))
	}
	return left + strings.Join(parts, mid) + right + "\n"
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", v)
}
```

### Example Output
```
┌────┬──────────┬───────┬────────────┐
│ id │ name     │ email │ created_at │
├────┼──────────┼───────┼────────────┤
│ 1  │ John Doe │ john@ │ 2024-01-01 │
│ 2  │ Jane Doe │ jane@ │ 2024-01-02 │
└────┴──────────┴───────┴────────────┘
```

## 2. Format Options

### Add format parameter to tools
```go
// Update tool schema
{
    "name": "query",
    "description": "Execute a MySQL query",
    "inputSchema": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "The SQL query to execute"
            },
            "format": {
                "type": "string",
                "enum": ["json", "table", "csv", "markdown"],
                "default": "table",
                "description": "Output format"
            }
        },
        "required": ["query"]
    }
}
```

### Format Implementations

#### CSV Format
```go
func FormatCSV(results []map[string]interface{}) string {
	if len(results) == 0 {
		return ""
	}
	
	var output strings.Builder
	writer := csv.NewWriter(&output)
	
	// Headers
	headers := make([]string, 0)
	for key := range results[0] {
		headers = append(headers, key)
	}
	writer.Write(headers)
	
	// Rows
	for _, row := range results {
		record := make([]string, len(headers))
		for i, header := range headers {
			record[i] = formatValue(row[header])
		}
		writer.Write(record)
	}
	
	writer.Flush()
	return output.String()
}
```

#### Markdown Format
```go
func FormatMarkdown(results []map[string]interface{}) string {
	if len(results) == 0 {
		return "No results"
	}
	
	var output strings.Builder
	
	// Headers
	headers := make([]string, 0)
	for key := range results[0] {
		headers = append(headers, key)
	}
	
	output.WriteString("| ")
	output.WriteString(strings.Join(headers, " | "))
	output.WriteString(" |\n")
	
	// Separator
	output.WriteString("|")
	for range headers {
		output.WriteString(" --- |")
	}
	output.WriteString("\n")
	
	// Rows
	for _, row := range results {
		output.WriteString("| ")
		values := make([]string, len(headers))
		for i, header := range headers {
			values[i] = formatValue(row[header])
		}
		output.WriteString(strings.Join(values, " | "))
		output.WriteString(" |\n")
	}
	
	return output.String()
}
```

## 3. Column Metadata

### Enhanced Schema Information
```go
type ColumnInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Nullable  bool   `json:"nullable"`
	Key       string `json:"key,omitempty"`
	Default   string `json:"default,omitempty"`
	Extra     string `json:"extra,omitempty"`
	MaxLength int    `json:"max_length,omitempty"`
}

func (c *Client) GetEnhancedSchema(tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_KEY,
			COLUMN_DEFAULT,
			EXTRA,
			CHARACTER_MAXIMUM_LENGTH
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`
	
	rows, err := c.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		var key, def, extra sql.NullString
		var maxLen sql.NullInt64
		
		err := rows.Scan(&col.Name, &col.Type, &nullable, &key, &def, &extra, &maxLen)
		if err != nil {
			return nil, err
		}
		
		col.Nullable = nullable == "YES"
		col.Key = key.String
		col.Default = def.String
		col.Extra = extra.String
		if maxLen.Valid {
			col.MaxLength = int(maxLen.Int64)
		}
		
		columns = append(columns, col)
	}
	
	return columns, nil
}
```

## 4. Query Statistics

### Performance Metrics
```go
type QueryStats struct {
	ExecutionTime   time.Duration          `json:"execution_time_ms"`
	RowsAffected    int64                  `json:"rows_affected"`
	RowsReturned    int                    `json:"rows_returned"`
	CacheHit        bool                   `json:"cache_hit"`
	QueryPlan       []map[string]interface{} `json:"query_plan,omitempty"`
}

func (c *Client) QueryWithStats(query string) ([]map[string]interface{}, *QueryStats, error) {
	stats := &QueryStats{}
	start := time.Now()
	
	// Check if it's a SELECT query for EXPLAIN
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
		// Get query plan
		explainRows, err := c.db.Query("EXPLAIN " + query)
		if err == nil {
			defer explainRows.Close()
			stats.QueryPlan = c.rowsToMaps(explainRows)
		}
	}
	
	// Execute actual query
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, stats, err
	}
	defer rows.Close()
	
	results := c.rowsToMaps(rows)
	
	stats.ExecutionTime = time.Since(start)
	stats.RowsReturned = len(results)
	
	return results, stats, nil
}
```

## 5. Pagination Support

### Implement LIMIT/OFFSET handling
```go
type PaginatedQuery struct {
	Query    string `json:"query"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

func (c *Client) QueryPaginated(pq PaginatedQuery) (*PaginatedResult, error) {
	// Count total rows
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS count_query", pq.Query)
	var totalRows int
	err := c.db.QueryRow(countQuery).Scan(&totalRows)
	if err != nil {
		return nil, err
	}
	
	// Execute paginated query
	offset := (pq.Page - 1) * pq.PageSize
	paginatedQuery := fmt.Sprintf("%s LIMIT %d OFFSET %d", pq.Query, pq.PageSize, offset)
	
	results, stats, err := c.QueryWithStats(paginatedQuery)
	if err != nil {
		return nil, err
	}
	
	return &PaginatedResult{
		Results:    results,
		Page:       pq.Page,
		PageSize:   pq.PageSize,
		TotalRows:  totalRows,
		TotalPages: (totalRows + pq.PageSize - 1) / pq.PageSize,
		Stats:      stats,
	}, nil
}

type PaginatedResult struct {
	Results    []map[string]interface{} `json:"results"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	TotalRows  int                      `json:"total_rows"`
	TotalPages int                      `json:"total_pages"`
	Stats      *QueryStats              `json:"stats"`
}
```

## 6. Smart Result Truncation

### Truncate large values intelligently
```go
type TruncateOptions struct {
	MaxCellLength   int  `json:"max_cell_length"`
	MaxJSONDepth    int  `json:"max_json_depth"`
	ShowEllipsis    bool `json:"show_ellipsis"`
	PreserveSQLTypes bool `json:"preserve_sql_types"`
}

func TruncateValue(value interface{}, opts TruncateOptions) string {
	str := fmt.Sprintf("%v", value)
	
	// Handle JSON strings
	if json.Valid([]byte(str)) {
		var v interface{}
		json.Unmarshal([]byte(str), &v)
		truncated := truncateJSON(v, opts.MaxJSONDepth)
		result, _ := json.Marshal(truncated)
		str = string(result)
	}
	
	// Truncate if too long
	if len(str) > opts.MaxCellLength && opts.MaxCellLength > 0 {
		if opts.ShowEllipsis {
			return str[:opts.MaxCellLength-3] + "..."
		}
		return str[:opts.MaxCellLength]
	}
	
	return str
}
```

## 7. Error Formatting

### Enhanced error messages with suggestions
```go
type EnhancedError struct {
	Code        int      `json:"code"`
	Message     string   `json:"message"`
	SQLState    string   `json:"sql_state,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
	Context     string   `json:"context,omitempty"`
	RelatedDocs []string `json:"related_docs,omitempty"`
}

func enhanceError(err error, query string) *EnhancedError {
	enhanced := &EnhancedError{
		Code:    -32603,
		Message: err.Error(),
	}
	
	// Parse MySQL errors
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		enhanced.Code = int(mysqlErr.Number)
		enhanced.SQLState = mysqlErr.SQLState
		
		// Add suggestions based on error type
		switch mysqlErr.Number {
		case 1146: // Table doesn't exist
			enhanced.Suggestion = "Use the 'tables' tool to list available tables"
		case 1054: // Unknown column
			enhanced.Suggestion = "Use the 'schema' tool to check column names"
		case 1064: // Syntax error
			enhanced.Suggestion = "Check SQL syntax near: " + extractErrorContext(query, err.Error())
		}
	}
	
	return enhanced
}
```

## 8. Result Summary

### Add summary statistics
```go
type ResultSummary struct {
	RowCount      int                    `json:"row_count"`
	ColumnCount   int                    `json:"column_count"`
	ColumnTypes   map[string]string      `json:"column_types"`
	NullCounts    map[string]int         `json:"null_counts"`
	NumericRanges map[string]NumericRange `json:"numeric_ranges,omitempty"`
}

type NumericRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

func GenerateSummary(results []map[string]interface{}) *ResultSummary {
	// Implementation to analyze results and generate summary
}
```

## Implementation Example

```go
func (s *MCPServer) handleQueryTool(id interface{}, args json.RawMessage) *Response {
	query := gjson.GetBytes(args, "query").String()
	format := gjson.GetBytes(args, "format").String()
	if format == "" {
		format = "table"
	}
	
	results, stats, err := s.mysqlClient.QueryWithStats(query)
	if err != nil {
		return s.errorResponse(id, enhanceError(err, query))
	}
	
	var formattedOutput string
	switch format {
	case "table":
		formatter := NewTableFormatter(results)
		formattedOutput = formatter.Render()
	case "csv":
		formattedOutput = FormatCSV(results)
	case "markdown":
		formattedOutput = FormatMarkdown(results)
	default:
		formattedOutput = formatResults(results)
	}
	
	summary := GenerateSummary(results)
	
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Query executed in %dms. %d rows returned.",
						stats.ExecutionTime.Milliseconds(), stats.RowsReturned),
				},
				{
					"type": "text",
					"text": formattedOutput,
				},
			},
			"metadata": map[string]interface{}{
				"stats":   stats,
				"summary": summary,
			},
		},
	}
}
```