package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sql-genius/pkg/models"
	"strings"
	"time"
)

// OllamaProvider Ollama 로컬 AI 제공자
type OllamaProvider struct {
	endpoint string
	model    string
	client   *http.Client
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewOllamaProvider Ollama 제공자 생성
func NewOllamaProvider(config models.AIConfig) (*OllamaProvider, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	model := config.Model
	if model == "" {
		model = "llama3.2" // 기본 모델
	}

	return &OllamaProvider{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

func (o *OllamaProvider) Name() string {
	return "Ollama"
}

func (o *OllamaProvider) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", o.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (o *OllamaProvider) generate(ctx context.Context, prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("JSON 마샬링 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.endpoint+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("요청 생성 실패: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("요청 실패: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("응답 읽기 실패: %w", err)
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("JSON 파싱 실패: %w", err)
	}

	return ollamaResp.Response, nil
}

func (o *OllamaProvider) GenerateQuery(ctx context.Context, req *models.QueryRequest) (*models.QueryResponse, error) {
	prompt := buildQueryPrompt(req)

	start := time.Now()
	response, err := o.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()

	query, explanation, tips := parseQueryResponse(response)

	return &models.QueryResponse{
		Query:       query,
		Explanation: explanation,
		Tips:        tips,
		ExecuteTime: elapsed,
	}, nil
}

func (o *OllamaProvider) OptimizeQuery(ctx context.Context, query string, schema *models.Schema) (*models.QueryResponse, error) {
	prompt := buildOptimizePrompt(query, schema)

	start := time.Now()
	response, err := o.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()

	optimized, explanation, tips := parseQueryResponse(response)

	return &models.QueryResponse{
		Query:       optimized,
		Explanation: explanation,
		Tips:        tips,
		ExecuteTime: elapsed,
	}, nil
}

func (o *OllamaProvider) ExplainQuery(ctx context.Context, query string) (string, error) {
	prompt := fmt.Sprintf(`다음 SQL 쿼리를 한국어로 설명해주세요:

%s

설명:`, query)

	return o.generate(ctx, prompt)
}

// buildQueryPrompt 쿼리 생성 프롬프트 구성
func buildQueryPrompt(req *models.QueryRequest) string {
	schemaStr := formatSchema(&req.Schema)

	prompt := fmt.Sprintf(`당신은 SQL 전문가입니다. 주어진 데이터베이스 스키마를 분석하고, 사용자 요청에 맞는 최적화된 SQL 쿼리를 생성해주세요.

## 데이터베이스 타입: %s

## 스키마 정보:
%s

## 사용자 요청:
%s

## 쿼리 타입: %s

## 요구사항:
1. 인덱스를 최대한 활용하세요
2. 불필요한 서브쿼리를 피하세요
3. 적절한 JOIN을 사용하세요
4. %s 문법에 맞게 작성하세요

## 응답 형식:
SQL:
(쿼리)

설명:
(간단한 설명)

최적화 팁:
- (팁1)
- (팁2)
`, req.Schema.DBType, schemaStr, req.Prompt, req.QueryType, req.Schema.DBType)

	return prompt
}

// buildOptimizePrompt 최적화 프롬프트 구성
func buildOptimizePrompt(query string, schema *models.Schema) string {
	schemaStr := formatSchema(schema)

	return fmt.Sprintf(`당신은 SQL 최적화 전문가입니다. 다음 쿼리를 분석하고 더 빠르게 실행될 수 있도록 최적화해주세요.

## 원본 쿼리:
%s

## 스키마 정보:
%s

## 응답 형식:
SQL:
(최적화된 쿼리)

설명:
(변경 사항 설명)

최적화 팁:
- (팁1)
- (팁2)
`, query, schemaStr)
}

// formatSchema 스키마를 문자열로 변환
func formatSchema(schema *models.Schema) string {
	var sb strings.Builder

	for _, table := range schema.Tables {
		sb.WriteString(fmt.Sprintf("테이블: %s\n", table.Name))
		sb.WriteString("컬럼:\n")
		for _, col := range table.Columns {
			flags := ""
			if col.IsPK {
				flags += " [PK]"
			}
			if col.IsFK {
				flags += " [FK]"
			}
			if col.IsUnique {
				flags += " [UNIQUE]"
			}
			sb.WriteString(fmt.Sprintf("  - %s %s%s\n", col.Name, col.Type, flags))
		}

		if len(table.Indexes) > 0 {
			sb.WriteString("인덱스:\n")
			for _, idx := range table.Indexes {
				sb.WriteString(fmt.Sprintf("  - %s (%s)\n", idx.Name, strings.Join(idx.Columns, ", ")))
			}
		}

		if len(table.ForeignKeys) > 0 {
			sb.WriteString("외래키:\n")
			for _, fk := range table.ForeignKeys {
				sb.WriteString(fmt.Sprintf("  - %s -> %s.%s\n", fk.Column, fk.RefTable, fk.RefColumn))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// parseQueryResponse AI 응답 파싱
func parseQueryResponse(response string) (query, explanation string, tips []string) {
	lines := strings.Split(response, "\n")

	var section string
	var queryLines, explainLines []string
	tips = []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "SQL:") {
			section = "sql"
			continue
		} else if strings.HasPrefix(trimmed, "설명:") {
			section = "explain"
			continue
		} else if strings.HasPrefix(trimmed, "최적화 팁:") {
			section = "tips"
			continue
		}

		switch section {
		case "sql":
			if trimmed != "" && trimmed != "```sql" && trimmed != "```" {
				queryLines = append(queryLines, line)
			}
		case "explain":
			if trimmed != "" {
				explainLines = append(explainLines, trimmed)
			}
		case "tips":
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "•") {
				tip := strings.TrimPrefix(trimmed, "-")
				tip = strings.TrimPrefix(tip, "•")
				tips = append(tips, strings.TrimSpace(tip))
			}
		}
	}

	query = strings.TrimSpace(strings.Join(queryLines, "\n"))
	explanation = strings.Join(explainLines, " ")

	// SQL 블록 마커 제거
	query = strings.TrimPrefix(query, "```sql")
	query = strings.TrimPrefix(query, "```")
	query = strings.TrimSuffix(query, "```")
	query = strings.TrimSpace(query)

	return
}

func (o *OllamaProvider) ValidateQuery(ctx context.Context, query string, schema *models.Schema) (*models.QueryValidation, error) {
	prompt := buildValidatePrompt(query, schema)

	start := time.Now()
	response, err := o.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()

	validation := parseValidationResponse(response, query)
	validation.AIResponseTime = elapsed

	return validation, nil
}

// buildValidatePrompt 쿼리 검증 프롬프트 구성
func buildValidatePrompt(query string, schema *models.Schema) string {
	schemaStr := formatSchema(schema)

	return fmt.Sprintf(`당신은 SQL 성능 분석 전문가입니다. 다음 쿼리를 분석하고 성능을 평가해주세요.

## 분석할 쿼리:
%s

## 데이터베이스 스키마:
%s

## 다음 항목들을 분석해주세요:
1. 쿼리 문법이 올바른지 (유효성)
2. 성능 점수 (0-100점)
3. 발견된 문제점 (type: error/warning/info)
4. 인덱스 활용 여부
5. 더 최적화된 쿼리가 있다면 제안
6. 예상 실행 계획

## 응답 형식 (반드시 이 형식을 따라주세요):
유효성: (true 또는 false)
점수: (0-100 숫자만)

문제점:
- [error] (문제 설명) | 위치: (위치) | 해결: (해결방안)
- [warning] (문제 설명) | 위치: (위치) | 해결: (해결방안)
- [info] (문제 설명) | 위치: (위치) | 해결: (해결방안)

인덱스 활용:
- (사용 가능한 인덱스1)
- (사용 가능한 인덱스2)

최적화된 쿼리:
(더 나은 쿼리가 있으면 작성, 없으면 "원본 쿼리가 최적입니다")

실행 계획:
(예상 실행 계획 설명)

예상 시간: (빠름/보통/느림)

개선 제안:
- (제안1)
- (제안2)
`, query, schemaStr)
}

// parseValidationResponse 검증 응답 파싱
func parseValidationResponse(response string, originalQuery string) *models.QueryValidation {
	validation := &models.QueryValidation{
		OriginalQuery:  originalQuery,
		IsValid:        true,
		Score:          50,
		Issues:         []models.Issue{},
		Suggestions:    []string{},
		IndexUsage:     []string{},
		OptimizedQuery: originalQuery,
	}

	lines := strings.Split(response, "\n")
	var section string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 섹션 감지
		if strings.HasPrefix(trimmed, "유효성:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "유효성:"))
			validation.IsValid = strings.Contains(strings.ToLower(val), "true") || strings.Contains(val, "유효")
			continue
		}
		if strings.HasPrefix(trimmed, "점수:") {
			scoreStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "점수:"))
			scoreStr = strings.TrimSuffix(scoreStr, "점")
			var score int
			fmt.Sscanf(scoreStr, "%d", &score)
			if score > 0 && score <= 100 {
				validation.Score = score
			}
			continue
		}
		if strings.HasPrefix(trimmed, "문제점:") {
			section = "issues"
			continue
		}
		if strings.HasPrefix(trimmed, "인덱스 활용:") {
			section = "indexes"
			continue
		}
		if strings.HasPrefix(trimmed, "최적화된 쿼리:") {
			section = "optimized"
			continue
		}
		if strings.HasPrefix(trimmed, "실행 계획:") {
			section = "plan"
			continue
		}
		if strings.HasPrefix(trimmed, "예상 시간:") {
			validation.EstimatedTime = strings.TrimSpace(strings.TrimPrefix(trimmed, "예상 시간:"))
			continue
		}
		if strings.HasPrefix(trimmed, "개선 제안:") {
			section = "suggestions"
			continue
		}

		// 섹션별 파싱
		switch section {
		case "issues":
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "•") {
				issue := parseIssue(trimmed)
				if issue.Message != "" {
					validation.Issues = append(validation.Issues, issue)
				}
			}
		case "indexes":
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "•") {
				idx := strings.TrimPrefix(trimmed, "-")
				idx = strings.TrimPrefix(idx, "•")
				idx = strings.TrimSpace(idx)
				if idx != "" && idx != "없음" {
					validation.IndexUsage = append(validation.IndexUsage, idx)
				}
			}
		case "optimized":
			if trimmed != "" && !strings.Contains(trimmed, "원본 쿼리가 최적") {
				if trimmed != "```sql" && trimmed != "```" {
					if validation.OptimizedQuery == originalQuery {
						validation.OptimizedQuery = trimmed
					} else {
						validation.OptimizedQuery += "\n" + trimmed
					}
				}
			}
		case "plan":
			if trimmed != "" {
				if validation.ExecutionPlan == "" {
					validation.ExecutionPlan = trimmed
				} else {
					validation.ExecutionPlan += " " + trimmed
				}
			}
		case "suggestions":
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "•") {
				sug := strings.TrimPrefix(trimmed, "-")
				sug = strings.TrimPrefix(sug, "•")
				sug = strings.TrimSpace(sug)
				if sug != "" {
					validation.Suggestions = append(validation.Suggestions, sug)
				}
			}
		}
	}

	// 최적화된 쿼리 정리
	validation.OptimizedQuery = strings.TrimPrefix(validation.OptimizedQuery, "```sql")
	validation.OptimizedQuery = strings.TrimSuffix(validation.OptimizedQuery, "```")
	validation.OptimizedQuery = strings.TrimSpace(validation.OptimizedQuery)
	if validation.OptimizedQuery == "" {
		validation.OptimizedQuery = originalQuery
	}

	return validation
}

// parseIssue 문제점 파싱
func parseIssue(line string) models.Issue {
	issue := models.Issue{Type: "info"}

	line = strings.TrimPrefix(line, "-")
	line = strings.TrimPrefix(line, "•")
	line = strings.TrimSpace(line)

	// 타입 감지
	if strings.Contains(line, "[error]") {
		issue.Type = "error"
		line = strings.Replace(line, "[error]", "", 1)
	} else if strings.Contains(line, "[warning]") {
		issue.Type = "warning"
		line = strings.Replace(line, "[warning]", "", 1)
	} else if strings.Contains(line, "[info]") {
		issue.Type = "info"
		line = strings.Replace(line, "[info]", "", 1)
	}

	// 파이프로 분리된 정보 파싱
	parts := strings.Split(line, "|")
	if len(parts) >= 1 {
		issue.Message = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		loc := strings.TrimPrefix(parts[1], "위치:")
		issue.Location = strings.TrimSpace(loc)
	}
	if len(parts) >= 3 {
		sug := strings.TrimPrefix(parts[2], "해결:")
		issue.Suggestion = strings.TrimSpace(sug)
	}

	return issue
}
