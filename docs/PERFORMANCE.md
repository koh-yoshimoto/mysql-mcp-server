# Performance Optimization Guide

This document outlines various approaches to improve the performance of the MySQL MCP Server.

## 1. Connection Pooling

### Current Issue
- New connection created for each server instance
- Connection overhead for each query

### Solution
```go
// mysql/pool.go
type ConnectionPool struct {
    connections chan *sql.DB
    config      *Config
    maxSize     int
}

func NewConnectionPool(config *Config, size int) (*ConnectionPool, error) {
    pool := &ConnectionPool{
        connections: make(chan *sql.DB, size),
        config:      config,
        maxSize:     size,
    }
    
    // Pre-create connections
    for i := 0; i < size; i++ {
        conn, err := createConnection(config)
        if err != nil {
            return nil, err
        }
        pool.connections <- conn
    }
    
    return pool, nil
}

func (p *ConnectionPool) Get() *sql.DB {
    return <-p.connections
}

func (p *ConnectionPool) Put(conn *sql.DB) {
    p.connections <- conn
}
```

## 2. Query Result Caching

### Implementation with TTL
```go
// cache/cache.go
type QueryCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry
    ttl     time.Duration
}

type CacheEntry struct {
    Result    []map[string]interface{}
    Timestamp time.Time
}

func (c *QueryCache) Get(query string) ([]map[string]interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, exists := c.entries[query]
    if !exists || time.Since(entry.Timestamp) > c.ttl {
        return nil, false
    }
    
    return entry.Result, true
}

func (c *QueryCache) Set(query string, result []map[string]interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.entries[query] = &CacheEntry{
        Result:    result,
        Timestamp: time.Now(),
    }
}
```

## 3. Streaming Large Results

### Current Issue
- All results loaded into memory
- Large result sets cause memory spikes

### Solution
```go
func (c *Client) QueryStream(query string, callback func(row map[string]interface{}) error) error {
    rows, err := c.db.Query(query)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    columns, err := rows.Columns()
    if err != nil {
        return err
    }
    
    for rows.Next() {
        values := make([]interface{}, len(columns))
        valuePtrs := make([]interface{}, len(columns))
        
        for i := range columns {
            valuePtrs[i] = &values[i]
        }
        
        if err := rows.Scan(valuePtrs...); err != nil {
            return err
        }
        
        row := make(map[string]interface{})
        for i, col := range columns {
            row[col] = values[i]
        }
        
        if err := callback(row); err != nil {
            return err
        }
    }
    
    return rows.Err()
}
```

## 4. Concurrent Request Processing

### Implement Request Pipeline
```go
type RequestPipeline struct {
    requests chan *Request
    workers  int
}

func (p *RequestPipeline) Start(handler func(*Request) *Response) {
    for i := 0; i < p.workers; i++ {
        go func() {
            for req := range p.requests {
                response := handler(req)
                // Send response back
            }
        }()
    }
}
```

## 5. Memory Optimization

### Use sync.Pool for Temporary Objects
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func formatResults(results []map[string]interface{}) string {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    
    encoder := json.NewEncoder(buf)
    encoder.SetIndent("", "  ")
    encoder.Encode(results)
    
    return buf.String()
}
```

## 6. Prepared Statements

### Cache and Reuse Prepared Statements
```go
type StatementCache struct {
    mu         sync.RWMutex
    statements map[string]*sql.Stmt
    db         *sql.DB
}

func (sc *StatementCache) Prepare(query string) (*sql.Stmt, error) {
    sc.mu.RLock()
    stmt, exists := sc.statements[query]
    sc.mu.RUnlock()
    
    if exists {
        return stmt, nil
    }
    
    sc.mu.Lock()
    defer sc.mu.Unlock()
    
    // Double-check
    if stmt, exists := sc.statements[query]; exists {
        return stmt, nil
    }
    
    stmt, err := sc.db.Prepare(query)
    if err != nil {
        return nil, err
    }
    
    sc.statements[query] = stmt
    return stmt, nil
}
```

## 7. Batch Operations

### Support Multiple Queries in Single Request
```go
type BatchQueryRequest struct {
    Queries []string `json:"queries"`
}

func (s *MCPServer) handleBatchQuery(queries []string) []interface{} {
    results := make([]interface{}, len(queries))
    
    // Use goroutines for parallel execution
    var wg sync.WaitGroup
    for i, query := range queries {
        wg.Add(1)
        go func(idx int, q string) {
            defer wg.Done()
            result, err := s.mysqlClient.Query(q)
            if err != nil {
                results[idx] = map[string]string{"error": err.Error()}
            } else {
                results[idx] = result
            }
        }(i, query)
    }
    
    wg.Wait()
    return results
}
```

## 8. Compression

### Compress Large Responses
```go
import "compress/gzip"

func (s *MCPServer) sendCompressedResponse(resp *Response) error {
    if s.compressionEnabled {
        var buf bytes.Buffer
        gz := gzip.NewWriter(&buf)
        
        data, err := json.Marshal(resp)
        if err != nil {
            return err
        }
        
        if _, err := gz.Write(data); err != nil {
            return err
        }
        
        if err := gz.Close(); err != nil {
            return err
        }
        
        // Send compressed data with header
        fmt.Fprintf(s.writer, "COMPRESSED:%s\n", base64.StdEncoding.EncodeToString(buf.Bytes()))
        return nil
    }
    
    return s.sendResponse(resp)
}
```

## 9. Monitoring and Metrics

### Add Performance Metrics
```go
type Metrics struct {
    QueryCount      int64
    CacheHits       int64
    CacheMisses     int64
    AvgResponseTime time.Duration
    ActiveConns     int32
}

func (m *Metrics) RecordQuery(duration time.Duration) {
    atomic.AddInt64(&m.QueryCount, 1)
    // Update average response time
}

func (m *Metrics) Report() map[string]interface{} {
    return map[string]interface{}{
        "queries":         atomic.LoadInt64(&m.QueryCount),
        "cache_hit_rate":  float64(m.CacheHits) / float64(m.CacheHits + m.CacheMisses),
        "avg_response_ms": m.AvgResponseTime.Milliseconds(),
        "active_conns":    atomic.LoadInt32(&m.ActiveConns),
    }
}
```

## 10. Configuration Tuning

### Recommended Settings
```yaml
# config.yaml
performance:
  connection_pool_size: 10
  cache_ttl: 300s
  cache_max_entries: 1000
  max_result_size: 10MB
  query_timeout: 30s
  compression_threshold: 1KB
  worker_threads: 4
  
mysql:
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
```

## Implementation Priority

1. **High Impact, Low Effort**
   - Connection pooling
   - Basic query caching
   - Prepared statements

2. **High Impact, Medium Effort**
   - Streaming for large results
   - Batch query support
   - Memory pooling

3. **Medium Impact, High Effort**
   - Full async processing
   - Advanced caching strategies
   - Compression

## Benchmarking

```go
// benchmark_test.go
func BenchmarkQuery(b *testing.B) {
    server := setupTestServer()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        server.handleQueryTool(1, []byte(`{"query": "SELECT 1"}`))
    }
}

func BenchmarkCachedQuery(b *testing.B) {
    server := setupTestServerWithCache()
    
    // Warm up cache
    server.handleQueryTool(1, []byte(`{"query": "SELECT 1"}`))
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        server.handleQueryTool(1, []byte(`{"query": "SELECT 1"}`))
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem
```