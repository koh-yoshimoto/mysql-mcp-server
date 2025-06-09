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
	
	headers := make([]string, 0)
	headerMap := make(map[string]int)
	
	// Extract headers maintaining order
	for key := range results[0] {
		headers = append(headers, key)
		headerMap[key] = len(headers) - 1
	}
	
	formatter := &TableFormatter{
		headers: headers,
		widths:  make([]int, len(headers)),
		rows:    make([][]string, 0, len(results)),
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
	output.WriteString("┌")
	for i, width := range f.widths {
		output.WriteString(strings.Repeat("─", width+2))
		if i < len(f.widths)-1 {
			output.WriteString("┬")
		}
	}
	output.WriteString("┐\n")
	
	// Headers
	output.WriteString("│")
	for i, header := range f.headers {
		output.WriteString(fmt.Sprintf(" %-*s │", f.widths[i], header))
	}
	output.WriteString("\n")
	
	// Header separator
	output.WriteString("├")
	for i, width := range f.widths {
		output.WriteString(strings.Repeat("─", width+2))
		if i < len(f.widths)-1 {
			output.WriteString("┼")
		}
	}
	output.WriteString("┤\n")
	
	// Rows
	for _, row := range f.rows {
		output.WriteString("│")
		for i, value := range row {
			output.WriteString(fmt.Sprintf(" %-*s │", f.widths[i], value))
		}
		output.WriteString("\n")
	}
	
	// Bottom border
	output.WriteString("└")
	for i, width := range f.widths {
		output.WriteString(strings.Repeat("─", width+2))
		if i < len(f.widths)-1 {
			output.WriteString("┴")
		}
	}
	output.WriteString("┘")
	
	return output.String()
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", v)
}