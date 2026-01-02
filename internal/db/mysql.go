package db

import (
	"context"
	"database/sql"
	"fmt"
	"sql-genius/pkg/models"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConnector MySQL 연결자
type MySQLConnector struct {
	BaseConnector
}

// NewMySQLConnector MySQL 연결자 생성
func NewMySQLConnector(config models.DBConfig) (*MySQLConnector, error) {
	return &MySQLConnector{
		BaseConnector: BaseConnector{config: config},
	}, nil
}

func (m *MySQLConnector) Connect(ctx context.Context) error {
	// 연결 타임아웃 60초, 읽기/쓰기 타임아웃 30초
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&timeout=60s&readTimeout=30s&writeTimeout=30s",
		m.config.User, m.config.Password, m.config.Host, m.config.Port, m.config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("MySQL 연결 실패: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// 연결 테스트 (최대 60초 대기)
	pingCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("MySQL Ping 실패: %w", err)
	}

	m.db = db
	return nil
}

func (m *MySQLConnector) ExtractSchema(ctx context.Context) (*models.Schema, error) {
	schema := &models.Schema{
		Database: m.config.Database,
		DBType:   models.MySQL,
		Tables:   []models.Table{},
	}

	// 테이블 목록 조회
	tables, err := m.getTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		table := models.Table{Name: tableName}

		// 컬럼 정보
		columns, err := m.getColumns(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Columns = columns

		// 인덱스 정보
		indexes, err := m.getIndexes(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Indexes = indexes

		// 외래키 정보
		fks, err := m.getForeignKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.ForeignKeys = fks

		// 기본키 정보
		pks, err := m.getPrimaryKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.PrimaryKey = pks

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func (m *MySQLConnector) getTables(ctx context.Context) ([]string, error) {
	query := "SHOW TABLES"
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (m *MySQLConnector) getColumns(ctx context.Context, table string) ([]models.Column, error) {
	query := `
		SELECT 
			COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, 
			COLUMN_KEY, EXTRA, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`

	rows, err := m.db.QueryContext(ctx, query, m.config.Database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []models.Column
	for rows.Next() {
		var col models.Column
		var nullable, columnKey, extra string
		var defaultVal, comment sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &columnKey, &extra, &comment); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "YES"
		col.IsPK = columnKey == "PRI"
		col.IsFK = columnKey == "MUL"
		col.IsUnique = columnKey == "UNI"
		col.IsAutoIncr = strings.Contains(extra, "auto_increment")
		if defaultVal.Valid {
			col.Default = defaultVal.String
		}
		if comment.Valid {
			col.Comment = comment.String
		}

		columns = append(columns, col)
	}
	return columns, nil
}

func (m *MySQLConnector) getIndexes(ctx context.Context, table string) ([]models.Index, error) {
	query := `
		SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := m.db.QueryContext(ctx, query, m.config.Database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*models.Index)
	for rows.Next() {
		var indexName, columnName, indexType string
		var nonUnique int

		if err := rows.Scan(&indexName, &columnName, &nonUnique, &indexType); err != nil {
			return nil, err
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &models.Index{
				Name:     indexName,
				Columns:  []string{columnName},
				IsUnique: nonUnique == 0,
				Type:     indexType,
			}
		}
	}

	var indexes []models.Index
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}
	return indexes, nil
}

func (m *MySQLConnector) getForeignKeys(ctx context.Context, table string) ([]models.FK, error) {
	query := `
		SELECT 
			CONSTRAINT_NAME, COLUMN_NAME, 
			REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? 
			AND REFERENCED_TABLE_NAME IS NOT NULL`

	rows, err := m.db.QueryContext(ctx, query, m.config.Database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []models.FK
	for rows.Next() {
		var fk models.FK
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.RefTable, &fk.RefColumn); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, nil
}

func (m *MySQLConnector) getPrimaryKeys(ctx context.Context, table string) ([]string, error) {
	query := `
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION`

	rows, err := m.db.QueryContext(ctx, query, m.config.Database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pks []string
	for rows.Next() {
		var pk string
		if err := rows.Scan(&pk); err != nil {
			return nil, err
		}
		pks = append(pks, pk)
	}
	return pks, nil
}

func (m *MySQLConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	start := time.Now()

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var resultRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make([]interface{}, len(columns))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		resultRows = append(resultRows, row)
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		Duration: time.Since(start).Milliseconds(),
	}, nil
}

func (m *MySQLConnector) Explain(ctx context.Context, query string) (string, error) {
	explainQuery := "EXPLAIN " + query
	rows, err := m.db.QueryContext(ctx, explainQuery)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var result strings.Builder

	// 헤더
	result.WriteString(strings.Join(columns, "\t") + "\n")
	result.WriteString(strings.Repeat("-", 80) + "\n")

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		var rowStr []string
		for _, v := range values {
			if v == nil {
				rowStr = append(rowStr, "NULL")
			} else if b, ok := v.([]byte); ok {
				rowStr = append(rowStr, string(b))
			} else {
				rowStr = append(rowStr, fmt.Sprintf("%v", v))
			}
		}
		result.WriteString(strings.Join(rowStr, "\t") + "\n")
	}

	return result.String(), nil
}
