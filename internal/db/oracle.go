package db

import (
	"context"
	"database/sql"
	"fmt"
	"sql-genius/pkg/models"
	"strings"
	"time"

	_ "github.com/sijms/go-ora/v2"
)

// OracleConnector Oracle 연결자
type OracleConnector struct {
	BaseConnector
}

// NewOracleConnector Oracle 연결자 생성
func NewOracleConnector(config models.DBConfig) (*OracleConnector, error) {
	return &OracleConnector{
		BaseConnector: BaseConnector{config: config},
	}, nil
}

func (o *OracleConnector) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		o.config.User, o.config.Password, o.config.Host, o.config.Port, o.config.Database)

	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return fmt.Errorf("Oracle 연결 실패: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("Oracle Ping 실패: %w", err)
	}

	o.db = db
	return nil
}

func (o *OracleConnector) ExtractSchema(ctx context.Context) (*models.Schema, error) {
	schema := &models.Schema{
		Database: o.config.Database,
		DBType:   models.Oracle,
		Tables:   []models.Table{},
	}

	tables, err := o.getTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		table := models.Table{Name: tableName}

		columns, err := o.getColumns(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Columns = columns

		indexes, err := o.getIndexes(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.Indexes = indexes

		fks, err := o.getForeignKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.ForeignKeys = fks

		pks, err := o.getPrimaryKeys(ctx, tableName)
		if err != nil {
			return nil, err
		}
		table.PrimaryKey = pks

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func (o *OracleConnector) getTables(ctx context.Context) ([]string, error) {
	query := `SELECT table_name FROM user_tables ORDER BY table_name`

	rows, err := o.db.QueryContext(ctx, query)
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

func (o *OracleConnector) getColumns(ctx context.Context, table string) ([]models.Column, error) {
	query := `
		SELECT 
			c.column_name, c.data_type, c.nullable, c.data_default,
			NVL((SELECT 'Y' FROM user_cons_columns ucc
				JOIN user_constraints uc ON ucc.constraint_name = uc.constraint_name
				WHERE uc.constraint_type = 'P' AND ucc.table_name = c.table_name 
				AND ucc.column_name = c.column_name AND ROWNUM = 1), 'N') as is_pk
		FROM user_tab_columns c
		WHERE c.table_name = :1
		ORDER BY c.column_id`

	rows, err := o.db.QueryContext(ctx, query, strings.ToUpper(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []models.Column
	for rows.Next() {
		var col models.Column
		var nullable, isPK string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &isPK); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "Y"
		col.IsPK = isPK == "Y"
		if defaultVal.Valid {
			col.Default = defaultVal.String
		}

		columns = append(columns, col)
	}
	return columns, nil
}

func (o *OracleConnector) getIndexes(ctx context.Context, table string) ([]models.Index, error) {
	query := `
		SELECT ui.index_name, uic.column_name, ui.uniqueness, ui.index_type
		FROM user_indexes ui
		JOIN user_ind_columns uic ON ui.index_name = uic.index_name
		WHERE ui.table_name = :1
		ORDER BY ui.index_name, uic.column_position`

	rows, err := o.db.QueryContext(ctx, query, strings.ToUpper(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*models.Index)
	for rows.Next() {
		var indexName, columnName, uniqueness, indexType string

		if err := rows.Scan(&indexName, &columnName, &uniqueness, &indexType); err != nil {
			return nil, err
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &models.Index{
				Name:     indexName,
				Columns:  []string{columnName},
				IsUnique: uniqueness == "UNIQUE",
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

func (o *OracleConnector) getForeignKeys(ctx context.Context, table string) ([]models.FK, error) {
	query := `
		SELECT 
			uc.constraint_name,
			ucc.column_name,
			(SELECT table_name FROM user_constraints WHERE constraint_name = uc.r_constraint_name) as ref_table,
			(SELECT column_name FROM user_cons_columns WHERE constraint_name = uc.r_constraint_name AND ROWNUM = 1) as ref_column
		FROM user_constraints uc
		JOIN user_cons_columns ucc ON uc.constraint_name = ucc.constraint_name
		WHERE uc.table_name = :1 AND uc.constraint_type = 'R'`

	rows, err := o.db.QueryContext(ctx, query, strings.ToUpper(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []models.FK
	for rows.Next() {
		var fk models.FK
		var refTable, refColumn sql.NullString
		if err := rows.Scan(&fk.Name, &fk.Column, &refTable, &refColumn); err != nil {
			return nil, err
		}
		if refTable.Valid {
			fk.RefTable = refTable.String
		}
		if refColumn.Valid {
			fk.RefColumn = refColumn.String
		}
		fks = append(fks, fk)
	}
	return fks, nil
}

func (o *OracleConnector) getPrimaryKeys(ctx context.Context, table string) ([]string, error) {
	query := `
		SELECT ucc.column_name
		FROM user_constraints uc
		JOIN user_cons_columns ucc ON uc.constraint_name = ucc.constraint_name
		WHERE uc.table_name = :1 AND uc.constraint_type = 'P'
		ORDER BY ucc.position`

	rows, err := o.db.QueryContext(ctx, query, strings.ToUpper(table))
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

func (o *OracleConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	start := time.Now()

	rows, err := o.db.QueryContext(ctx, query)
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

func (o *OracleConnector) Explain(ctx context.Context, query string) (string, error) {
	// Oracle EXPLAIN PLAN
	explainQuery := fmt.Sprintf("EXPLAIN PLAN FOR %s", query)
	_, err := o.db.ExecContext(ctx, explainQuery)
	if err != nil {
		return "", err
	}

	planQuery := `SELECT plan_table_output FROM TABLE(DBMS_XPLAN.DISPLAY())`
	rows, err := o.db.QueryContext(ctx, planQuery)
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

