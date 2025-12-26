package schema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sql-genius/pkg/models"
	"strings"
)

// Parser 스키마 파서
type Parser struct{}

// NewParser 파서 생성
func NewParser() *Parser {
	return &Parser{}
}

// ParseJSON JSON 형식 스키마 파싱
func (p *Parser) ParseJSON(data []byte) (*models.Schema, error) {
	var schema models.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %w", err)
	}
	return &schema, nil
}

// ParseDDL DDL (CREATE TABLE) 문에서 스키마 파싱
func (p *Parser) ParseDDL(ddl string, dbType models.DBType) (*models.Schema, error) {
	schema := &models.Schema{
		DBType: dbType,
		Tables: []models.Table{},
	}

	// CREATE TABLE 문 찾기
	tablePattern := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\s*\(([\s\S]*?)\)`)
	matches := tablePattern.FindAllStringSubmatch(ddl, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		tableName := match[1]
		columnsDef := match[2]

		table := models.Table{
			Name:    tableName,
			Columns: []models.Column{},
		}

		// 컬럼 파싱
		columns := p.parseColumns(columnsDef, dbType)
		table.Columns = columns

		// PRIMARY KEY 파싱
		pks := p.parsePrimaryKey(columnsDef)
		table.PrimaryKey = pks

		// FOREIGN KEY 파싱
		fks := p.parseForeignKeys(columnsDef)
		table.ForeignKeys = fks

		schema.Tables = append(schema.Tables, table)
	}

	// CREATE INDEX 문 파싱
	p.parseIndexes(ddl, schema)

	return schema, nil
}

func (p *Parser) parseColumns(columnsDef string, dbType models.DBType) []models.Column {
	var columns []models.Column

	// 줄 단위로 분리
	lines := strings.Split(columnsDef, ",")

	columnPattern := regexp.MustCompile(`^\s*[` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\s+(\w+(?:\([^)]+\))?)\s*(.*)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 제약조건 라인 스킵
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "PRIMARY KEY") ||
			strings.HasPrefix(upperLine, "FOREIGN KEY") ||
			strings.HasPrefix(upperLine, "CONSTRAINT") ||
			strings.HasPrefix(upperLine, "INDEX") ||
			strings.HasPrefix(upperLine, "KEY") ||
			strings.HasPrefix(upperLine, "UNIQUE") {
			continue
		}

		match := columnPattern.FindStringSubmatch(line)
		if len(match) < 3 {
			continue
		}

		col := models.Column{
			Name: match[1],
			Type: match[2],
		}

		constraints := strings.ToUpper(match[3])

		// NOT NULL 확인
		col.Nullable = !strings.Contains(constraints, "NOT NULL")

		// PRIMARY KEY 확인
		col.IsPK = strings.Contains(constraints, "PRIMARY KEY")

		// UNIQUE 확인
		col.IsUnique = strings.Contains(constraints, "UNIQUE")

		// AUTO_INCREMENT / SERIAL / IDENTITY 확인
		col.IsAutoIncr = strings.Contains(constraints, "AUTO_INCREMENT") ||
			strings.Contains(strings.ToUpper(col.Type), "SERIAL") ||
			strings.Contains(constraints, "IDENTITY")

		// DEFAULT 값 파싱
		defaultPattern := regexp.MustCompile(`(?i)DEFAULT\s+([^\s,]+)`)
		if defaultMatch := defaultPattern.FindStringSubmatch(match[3]); len(defaultMatch) > 1 {
			col.Default = defaultMatch[1]
		}

		// COMMENT 파싱 (MySQL)
		commentPattern := regexp.MustCompile(`(?i)COMMENT\s+'([^']*)'`)
		if commentMatch := commentPattern.FindStringSubmatch(match[3]); len(commentMatch) > 1 {
			col.Comment = commentMatch[1]
		}

		columns = append(columns, col)
	}

	return columns
}

func (p *Parser) parsePrimaryKey(columnsDef string) []string {
	pkPattern := regexp.MustCompile(`(?i)PRIMARY\s+KEY\s*\(([^)]+)\)`)
	match := pkPattern.FindStringSubmatch(columnsDef)
	if len(match) < 2 {
		return nil
	}

	cols := strings.Split(match[1], ",")
	var pks []string
	for _, col := range cols {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "`\"'[]")
		if col != "" {
			pks = append(pks, col)
		}
	}
	return pks
}

func (p *Parser) parseForeignKeys(columnsDef string) []models.FK {
	fkPattern := regexp.MustCompile(`(?i)(?:CONSTRAINT\s+(\w+)\s+)?FOREIGN\s+KEY\s*\([` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\)\s*REFERENCES\s+[` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\s*\([` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\)`)
	matches := fkPattern.FindAllStringSubmatch(columnsDef, -1)

	var fks []models.FK
	for _, match := range matches {
		fk := models.FK{
			Column:    match[2],
			RefTable:  match[3],
			RefColumn: match[4],
		}
		if match[1] != "" {
			fk.Name = match[1]
		} else {
			fk.Name = fmt.Sprintf("fk_%s_%s", match[2], match[3])
		}
		fks = append(fks, fk)
	}
	return fks
}

func (p *Parser) parseIndexes(ddl string, schema *models.Schema) {
	indexPattern := regexp.MustCompile(`(?i)CREATE\s+(UNIQUE\s+)?INDEX\s+[` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\s+ON\s+[` + "`" + `"'\[]?(\w+)[` + "`" + `"'\]]?\s*\(([^)]+)\)`)
	matches := indexPattern.FindAllStringSubmatch(ddl, -1)

	for _, match := range matches {
		if len(match) < 5 {
			continue
		}

		isUnique := strings.TrimSpace(match[1]) != ""
		indexName := match[2]
		tableName := match[3]
		columnStr := match[4]

		cols := strings.Split(columnStr, ",")
		var columns []string
		for _, col := range cols {
			col = strings.TrimSpace(col)
			col = strings.Trim(col, "`\"'[]")
			// ASC/DESC 제거
			col = strings.Split(col, " ")[0]
			if col != "" {
				columns = append(columns, col)
			}
		}

		idx := models.Index{
			Name:     indexName,
			Columns:  columns,
			IsUnique: isUnique,
			Type:     "BTREE",
		}

		// 해당 테이블에 인덱스 추가
		for i := range schema.Tables {
			if strings.EqualFold(schema.Tables[i].Name, tableName) {
				schema.Tables[i].Indexes = append(schema.Tables[i].Indexes, idx)
				break
			}
		}
	}
}

// ToJSON 스키마를 JSON으로 변환
func (p *Parser) ToJSON(schema *models.Schema) ([]byte, error) {
	return json.MarshalIndent(schema, "", "  ")
}

// GenerateDDL 스키마에서 DDL 생성
func (p *Parser) GenerateDDL(schema *models.Schema) string {
	var sb strings.Builder

	for _, table := range schema.Tables {
		sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", p.quote(table.Name, schema.DBType)))

		var columnDefs []string
		for _, col := range table.Columns {
			colDef := fmt.Sprintf("  %s %s", p.quote(col.Name, schema.DBType), col.Type)

			if !col.Nullable {
				colDef += " NOT NULL"
			}
			if col.IsAutoIncr {
				switch schema.DBType {
				case models.MySQL:
					colDef += " AUTO_INCREMENT"
				case models.SQLServer:
					colDef += " IDENTITY(1,1)"
				}
			}
			if col.Default != "" {
				colDef += " DEFAULT " + col.Default
			}

			columnDefs = append(columnDefs, colDef)
		}

		// PRIMARY KEY
		if len(table.PrimaryKey) > 0 {
			pkCols := make([]string, len(table.PrimaryKey))
			for i, pk := range table.PrimaryKey {
				pkCols[i] = p.quote(pk, schema.DBType)
			}
			columnDefs = append(columnDefs, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
		}

		// FOREIGN KEYS
		for _, fk := range table.ForeignKeys {
			fkDef := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
				p.quote(fk.Name, schema.DBType),
				p.quote(fk.Column, schema.DBType),
				p.quote(fk.RefTable, schema.DBType),
				p.quote(fk.RefColumn, schema.DBType))
			columnDefs = append(columnDefs, fkDef)
		}

		sb.WriteString(strings.Join(columnDefs, ",\n"))
		sb.WriteString("\n);\n\n")

		// INDEXES
		for _, idx := range table.Indexes {
			if idx.Name == "PRIMARY" {
				continue
			}
			unique := ""
			if idx.IsUnique {
				unique = "UNIQUE "
			}
			idxCols := make([]string, len(idx.Columns))
			for i, col := range idx.Columns {
				idxCols[i] = p.quote(col, schema.DBType)
			}
			sb.WriteString(fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);\n",
				unique, p.quote(idx.Name, schema.DBType),
				p.quote(table.Name, schema.DBType),
				strings.Join(idxCols, ", ")))
		}
	}

	return sb.String()
}

func (p *Parser) quote(name string, dbType models.DBType) string {
	switch dbType {
	case models.MySQL:
		return "`" + name + "`"
	case models.PostgreSQL:
		return `"` + name + `"`
	case models.SQLServer:
		return "[" + name + "]"
	case models.Oracle:
		return `"` + strings.ToUpper(name) + `"`
	default:
		return name
	}
}

