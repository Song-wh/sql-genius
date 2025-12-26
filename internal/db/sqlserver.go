package db

import (
	"context"
	"database/sql"
	"fmt"
	"sql-genius/pkg/models"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

// SQLServerConnector SQL Server 연결자
type SQLServerConnector struct {
	BaseConnector
}

// NewSQLServerConnector SQL Server 연결자 생성
func NewSQLServerConnector(config models.DBConfig) (*SQLServerConnector, error) {
	return &SQLServerConnector{
		BaseConnector: BaseConnector{config: config},
	}, nil
}

func (s *SQLServerConnector) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;database=%s",
		s.config.Host, s.config.Port, s.config.User, s.config.Password, s.config.Database)

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return fmt.Errorf("SQL Server 연결 실패: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("SQL Server Ping 실패: %w", err)
	}

	s.db = db
	return nil
}

func (s *SQLServerConnector) ExtractSchema(ctx context.Context) (*models.Schema, error) {
	schema := &models.Schema{
		Database: s.config.Database,
		DBType:   models.SQLServer,
		Tables:   []models.Table{},
	}

	tables, err := s.getTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		table := models.Table{Name: tableName}

		columns, err := s.getColumns(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Columns = columns

		indexes, err := s.getIndexes(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Indexes = indexes

		fks, err := s.getForeignKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.ForeignKeys = fks

		pks, err := s.getPrimaryKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.PrimaryKey = pks

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func (s *SQLServerConnector) getTables(ctx context.Context) ([]string, error) {
	query := `
		SELECT TABLE_NAME 
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_CATALOG = DB_NAME()
		ORDER BY TABLE_NAME`

	rows, err := s.db.QueryContext(ctx, query)
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

func (s *SQLServerConnector) getColumns(ctx context.Context, table string) ([]models.Column, error) {
	query := `
		SELECT 
			c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE, c.COLUMN_DEFAULT,
			CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END as is_pk,
			COLUMNPROPERTY(OBJECT_ID(c.TABLE_SCHEMA + '.' + c.TABLE_NAME), c.COLUMN_NAME, 'IsIdentity') as is_identity
		FROM INFORMATION_SCHEMA.COLUMNS c
		LEFT JOIN (
			SELECT ku.COLUMN_NAME
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
			JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME
			WHERE tc.TABLE_NAME = @p1 AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
		) pk ON c.COLUMN_NAME = pk.COLUMN_NAME
		WHERE c.TABLE_NAME = @p1
		ORDER BY c.ORDINAL_POSITION`

	rows, err := s.db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []models.Column
	for rows.Next() {
		var col models.Column
		var nullable string
		var defaultVal sql.NullString
		var isPK, isIdentity int

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &isPK, &isIdentity); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "YES"
		col.IsPK = isPK == 1
		col.IsAutoIncr = isIdentity == 1
		if defaultVal.Valid {
			col.Default = defaultVal.String
		}

		columns = append(columns, col)
	}
	return columns, nil
}

func (s *SQLServerConnector) getIndexes(ctx context.Context, table string) ([]models.Index, error) {
	query := `
		SELECT 
			i.name as index_name,
			c.name as column_name,
			i.is_unique,
			i.type_desc as index_type
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE i.object_id = OBJECT_ID(@p1) AND i.name IS NOT NULL
		ORDER BY i.name, ic.key_ordinal`

	rows, err := s.db.QueryContext(ctx, query, table)
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

func (s *SQLServerConnector) getForeignKeys(ctx context.Context, table string) ([]models.FK, error) {
	query := `
		SELECT 
			fk.name as constraint_name,
			COL_NAME(fkc.parent_object_id, fkc.parent_column_id) as column_name,
			OBJECT_NAME(fkc.referenced_object_id) as ref_table,
			COL_NAME(fkc.referenced_object_id, fkc.referenced_column_id) as ref_column
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		WHERE fk.parent_object_id = OBJECT_ID(@p1)`

	rows, err := s.db.QueryContext(ctx, query, table)
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

func (s *SQLServerConnector) getPrimaryKeys(ctx context.Context, table string) ([]string, error) {
	query := `
		SELECT ku.COLUMN_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME
		WHERE tc.TABLE_NAME = @p1 AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
		ORDER BY ku.ORDINAL_POSITION`

	rows, err := s.db.QueryContext(ctx, query, table)
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

func (s *SQLServerConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	start := time.Now()

	rows, err := s.db.QueryContext(ctx, query)
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

func (s *SQLServerConnector) Explain(ctx context.Context, query string) (string, error) {
	// SQL Server: SET SHOWPLAN_TEXT ON
	_, err := s.db.ExecContext(ctx, "SET SHOWPLAN_TEXT ON")
	if err != nil {
		return "", err
	}
	defer s.db.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF")

	rows, err := s.db.QueryContext(ctx, query)
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

