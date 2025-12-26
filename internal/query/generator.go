package query

import (
	"context"
	"sql-genius/internal/ai"
	"sql-genius/pkg/models"
)

// Generator 쿼리 생성기
type Generator struct {
	aiProvider ai.Provider
	schema     *models.Schema
}

// NewGenerator 쿼리 생성기 생성
func NewGenerator(provider ai.Provider, schema *models.Schema) *Generator {
	return &Generator{
		aiProvider: provider,
		schema:     schema,
	}
}

// Generate 자연어로 쿼리 생성
func (g *Generator) Generate(ctx context.Context, prompt string, queryType string) (*models.QueryResponse, error) {
	req := &models.QueryRequest{
		Prompt:    prompt,
		Schema:    *g.schema,
		QueryType: queryType,
		Optimize:  true,
	}

	return g.aiProvider.GenerateQuery(ctx, req)
}

// GenerateSelect SELECT 쿼리 생성
func (g *Generator) GenerateSelect(ctx context.Context, prompt string) (*models.QueryResponse, error) {
	return g.Generate(ctx, prompt, "SELECT")
}

// GenerateInsert INSERT 쿼리 생성
func (g *Generator) GenerateInsert(ctx context.Context, prompt string) (*models.QueryResponse, error) {
	return g.Generate(ctx, prompt, "INSERT")
}

// GenerateUpdate UPDATE 쿼리 생성
func (g *Generator) GenerateUpdate(ctx context.Context, prompt string) (*models.QueryResponse, error) {
	return g.Generate(ctx, prompt, "UPDATE")
}

// GenerateDelete DELETE 쿼리 생성
func (g *Generator) GenerateDelete(ctx context.Context, prompt string) (*models.QueryResponse, error) {
	return g.Generate(ctx, prompt, "DELETE")
}

// GenerateAlter ALTER 쿼리 생성
func (g *Generator) GenerateAlter(ctx context.Context, prompt string) (*models.QueryResponse, error) {
	return g.Generate(ctx, prompt, "ALTER")
}

// GenerateCreate CREATE 쿼리 생성
func (g *Generator) GenerateCreate(ctx context.Context, prompt string) (*models.QueryResponse, error) {
	return g.Generate(ctx, prompt, "CREATE")
}

// Optimize 기존 쿼리 최적화
func (g *Generator) Optimize(ctx context.Context, query string) (*models.QueryResponse, error) {
	return g.aiProvider.OptimizeQuery(ctx, query, g.schema)
}

// Explain 쿼리 설명
func (g *Generator) Explain(ctx context.Context, query string) (string, error) {
	return g.aiProvider.ExplainQuery(ctx, query)
}

// SetSchema 스키마 설정
func (g *Generator) SetSchema(schema *models.Schema) {
	g.schema = schema
}

// GetSchema 현재 스키마 조회
func (g *Generator) GetSchema() *models.Schema {
	return g.schema
}

