package db

import (
	"context"
	"database/sql"
	"fmt"
	"sql-genius/pkg/models"
)

// Connector 데이터베이스 연결 인터페이스
type Connector interface {
	// Connect 데이터베이스 연결
	Connect(ctx context.Context) error

	// Close 연결 종료
	Close() error

	// Ping 연결 상태 확인
	Ping(ctx context.Context) error

	// ExtractSchema 스키마 추출
	ExtractSchema(ctx context.Context) (*models.Schema, error)

	// ExecuteQuery 쿼리 실행 (결과 반환)
	ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)

	// Explain 실행 계획 조회
	Explain(ctx context.Context, query string) (string, error)

	// GetDB 내부 DB 객체 반환
	GetDB() *sql.DB

	// Type 데이터베이스 타입 반환
	Type() models.DBType
}

// QueryResult 쿼리 실행 결과
type QueryResult struct {
	Columns      []string        `json:"columns"`
	Rows         [][]interface{} `json:"rows"`
	RowsAffected int64           `json:"rows_affected"`
	Duration     int64           `json:"duration"` // ms
}

// NewConnector DB 연결자 생성
func NewConnector(config models.DBConfig) (Connector, error) {
	switch config.Type {
	case models.MySQL:
		return NewMySQLConnector(config)
	case models.PostgreSQL:
		return NewPostgresConnector(config)
	case models.Oracle:
		return NewOracleConnector(config)
	case models.SQLServer:
		return NewSQLServerConnector(config)
	default:
		return nil, fmt.Errorf("지원하지 않는 데이터베이스 타입: %s", config.Type)
	}
}

// BaseConnector 공통 기능
type BaseConnector struct {
	db     *sql.DB
	config models.DBConfig
}

func (b *BaseConnector) GetDB() *sql.DB {
	return b.db
}

func (b *BaseConnector) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

func (b *BaseConnector) Ping(ctx context.Context) error {
	return b.db.PingContext(ctx)
}

func (b *BaseConnector) Type() models.DBType {
	return b.config.Type
}

