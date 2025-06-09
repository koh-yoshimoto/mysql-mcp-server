package format

import (
	"strings"
)

func FormatMarkdown(results []map[string]interface{}) string {
	if len(results) == 0 {
		return "No results"
	}
	
	var output strings.Builder
	
	// Extract headers
	headers := make([]string, 0)
	for key := range results[0] {
		headers = append(headers, key)
	}
	
	// Write header row
	output.WriteString("| ")
	output.WriteString(strings.Join(headers, " | "))
	output.WriteString(" |\n")
	
	// Write separator
	output.WriteString("|")
	for range headers {
		output.WriteString(" --- |")
	}
	output.WriteString("\n")
	
	// Write data rows
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