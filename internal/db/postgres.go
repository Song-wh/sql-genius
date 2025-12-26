package db

import (
	"context"
	"database/sql"
	"fmt"
	"sql-genius/pkg/models"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConnector PostgreSQL 연결자
type PostgresConnector struct {
	BaseConnector
}

// NewPostgresConnector PostgreSQL 연결자 생성
func NewPostgresConnector(config models.DBConfig) (*PostgresConnector, error) {
	return &PostgresConnector{
		BaseConnector: BaseConnector{config: config},
	}, nil
}

func (p *PostgresConnector) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		p.config.Host, p.config.Port, p.config.User, p.config.Password, p.config.Database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("PostgreSQL 연결 실패: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("PostgreSQL Ping 실패: %w", err)
	}

	p.db = db
	return nil
}

func (p *PostgresConnector) ExtractSchema(ctx context.Context) (*models.Schema, error) {
	schema := &models.Schema{
		Database: p.config.Database,
		DBType:   models.PostgreSQL,
		Tables:   []models.Table{},
	}

	tables, err := p.getTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		table := models.Table{Name: tableName}

		columns, err := p.getColumns(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Columns = columns

		indexes, err := p.getIndexes(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Indexes = indexes

		fks, err := p.getForeignKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.ForeignKeys = fks

		pks, err := p.getPrimaryKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.PrimaryKey = pks

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func (p *PostgresConnector) getTables(ctx context.Context) ([]string, error) {
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := p.db.QueryContext(ctx, query)
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

func (p *PostgresConnector) getColumns(ctx context.Context, table string) ([]models.Column, error) {
	query := `
		SELECT 
			c.column_name, c.data_type, c.is_nullable, c.column_default,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_pk,
			CASE WHEN fk.column_name IS NOT NULL THEN true ELSE false END as is_fk
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name 
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		LEFT JOIN (
			SELECT kcu.column_name 
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'FOREIGN KEY'
		) fk ON c.column_name = fk.column_name
		WHERE c.table_name = $1 AND c.table_schema = 'public'
		ORDER BY c.ordinal_position`

	rows, err := p.db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []models.Column
	for rows.Next() {
		var col models.Column
		var nullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &col.IsPK, &col.IsFK); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "YES"
		if defaultVal.Valid {
			col.Default = defaultVal.String
			if strings.Contains(col.Default, "nextval") {
				col.IsAutoIncr = true
			}
		}

		columns = append(columns, col)
	}
	return columns, nil
}

func (p *PostgresConnector) getIndexes(ctx context.Context, table string) ([]models.Index, error) {
	query := `
		SELECT 
			i.relname as index_name,
			a.attname as column_name,
			ix.indisunique as is_unique,
			am.amname as index_type
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1 AND t.relkind = 'r'
		ORDER BY i.relname, a.attnum`

	rows, err := p.db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*models.Index)
	for rows.Next() {
		var indexName, columnName, indexType string
		var isUnique bool

		if err := rows.Scan(&indexName, &columnName, &isUnique, &indexType); err != nil {
			return nil, err
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &models.Index{
				Name:     indexName,
				Columns:  []string{columnName},
				IsUnique: isUnique,
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

func (p *PostgresConnector) getForeignKeys(ctx context.Context, table string) ([]models.FK, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS ref_table,
			ccu.column_name AS ref_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu 
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.table_name = $1 AND tc.constraint_type = 'FOREIGN KEY'`

	rows, err := p.db.QueryContext(ctx, query, table)
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

func (p *PostgresConnector) getPrimaryKeys(ctx context.Context, table string) ([]string, error) {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position`

	rows, err := p.db.QueryContext(ctx, query, table)
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

func (p *PostgresConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	start := time.Now()

	rows, err := p.db.QueryContext(ctx, query)
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

func (p *PostgresConnector) Explain(ctx context.Context, query string) (string, error) {
	explainQuery := "EXPLAIN ANALYZE " + query
	rows, err := p.db.QueryContext(ctx, explainQuery)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var result strings.Builder
	for rows.Next() {
		var line string
		rows.Scan(&line)
		result.WriteString(line + "\n")
	}

	return result.String(), nil
}

