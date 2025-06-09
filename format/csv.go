package format

import (
	"encoding/csv"
	"strings"
)

func FormatCSV(results []map[string]interface{}) string {
	if len(results) == 0 {
		return ""
	}
	
	var output strings.Builder
	writer := csv.NewWriter(&output)
	
	// Extract headers
	headers := make([]string, 0)
	for key := range results[0] {
		headers = append(headers, key)
	}
	writer.Write(headers)
	
	// Write rows
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