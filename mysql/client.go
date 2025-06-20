package mysql

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Client struct {
	db *sql.DB
}

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

func NewClient(config *Config) (*Client, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) Query(query string) ([]map[string]interface{}, error) {
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))

	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			row[col] = v
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}

func (c *Client) GetTables() ([]string, error) {
	rows, err := c.db.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to show tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return tables, nil
}

func (c *Client) GetTableSchema(tableName string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("DESCRIBE `%s`", strings.ReplaceAll(tableName, "`", "``"))
	return c.Query(query)
}

// Execute executes a non-SELECT query (INSERT, UPDATE, DELETE, etc.)
func (c *Client) Execute(query string) (sql.Result, error) {
	result, err := c.db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}
	return result, nil
}

// ExecuteInTransaction executes a query within a transaction and returns the affected rows
// The transaction is always rolled back, making this perfect for dry-run operations
func (c *Client) ExecuteInTransaction(query string) (int64, error) {
	// Start transaction
	tx, err := c.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	// Ensure we always rollback
	defer tx.Rollback()
	
	// Execute the query
	result, err := tx.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("execution failed: %w", err)
	}
	
	// Get affected rows
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}
	
	// Transaction will be rolled back by defer
	return affected, nil
}

// CanUseTransaction checks if a query can be executed in a transaction
// Some statements like CREATE, DROP, ALTER cannot be rolled back in MySQL
func (c *Client) CanUseTransaction(query string) bool {
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	
	// DDL statements that cannot be rolled back in MySQL
	nonTransactionalStatements := []string{
		"CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME",
		"ANALYZE", "CHECK", "OPTIMIZE", "REPAIR",
		"LOCK", "UNLOCK", "SET", "START", "COMMIT", "ROLLBACK",
	}
	
	for _, stmt := range nonTransactionalStatements {
		if strings.HasPrefix(upperQuery, stmt+" ") || upperQuery == stmt {
			return false
		}
	}
	
	return true
}