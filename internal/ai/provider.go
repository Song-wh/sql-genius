package ai

import (
	"context"
	"sql-genius/pkg/models"
)

// Provider AI 제공자 인터페이스
type Provider interface {
	// GenerateQuery 자연어를 SQL 쿼리로 변환
	GenerateQuery(ctx context.Context, req *models.QueryRequest) (*models.QueryResponse, error)
	
	// OptimizeQuery 쿼리 최적화 제안
	OptimizeQuery(ctx context.Context, query string, schema *models.Schema) (*models.QueryResponse, error)
	
	// ExplainQuery 쿼리 설명
	ExplainQuery(ctx context.Context, query string) (string, error)
	
	// ValidateQuery 쿼리 검증 및 최적화 제안
	ValidateQuery(ctx context.Context, query string, schema *models.Schema) (*models.QueryValidation, error)
	
	// Name 제공자 이름
	Name() string
	
	// IsAvailable 사용 가능 여부 확인
	IsAvailable(ctx context.Context) bool
}

// NewProvider AI 제공자 생성
func NewProvider(config models.AIConfig) (Provider, error) {
	switch config.Provider {
	case models.Ollama:
		return NewOllamaProvider(config)
	case models.Groq:
		return NewGroqProvider(config)
	default:
		return NewOllamaProvider(config) // 기본값: Ollama
	}
}

