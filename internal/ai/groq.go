package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sql-genius/pkg/models"
	"time"
)

// GroqProvider Groq AI 제공자 (무료, 초고속)
type GroqProvider struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewGroqProvider Groq 제공자 생성
func NewGroqProvider(config models.AIConfig) (*GroqProvider, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.groq.com/openai/v1"
	}

	model := config.Model
	if model == "" {
		model = "llama-3.3-70b-versatile" // 무료, 빠름
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("Groq API 키가 필요합니다")
	}

	return &GroqProvider{
		endpoint: endpoint,
		model:    model,
		apiKey:   config.APIKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (g *GroqProvider) Name() string {
	return "Groq"
}

func (g *GroqProvider) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.endpoint+"/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (g *GroqProvider) generate(ctx context.Context, prompt string) (string, error) {
	reqBody := groqRequest{
		Model: g.model,
		Messages: []groqMessage{
			{
				Role:    "system",
				Content: "당신은 SQL 전문가입니다. 사용자 요청에 맞는 최적화된 SQL 쿼리를 생성합니다.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   2048,
		Temperature: 0.1, // 낮은 temperature로 일관된 결과
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("JSON 마샬링 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", g.endpoint+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("요청 생성 실패: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("요청 실패: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("응답 읽기 실패: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 오류 (상태 코드: %d): %s", resp.StatusCode, string(body))
	}

	var groqResp groqResponse
	if err := json.Unmarshal(body, &groqResp); err != nil {
		return "", fmt.Errorf("JSON 파싱 실패: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("응답이 비어있습니다")
	}

	return groqResp.Choices[0].Message.Content, nil
}

func (g *GroqProvider) GenerateQuery(ctx context.Context, req *models.QueryRequest) (*models.QueryResponse, error) {
	prompt := buildQueryPrompt(req)

	start := time.Now()
	response, err := g.generate(ctx, prompt)
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

func (g *GroqProvider) OptimizeQuery(ctx context.Context, query string, schema *models.Schema) (*models.QueryResponse, error) {
	prompt := buildOptimizePrompt(query, schema)

	start := time.Now()
	response, err := g.generate(ctx, prompt)
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

func (g *GroqProvider) ExplainQuery(ctx context.Context, query string) (string, error) {
	prompt := fmt.Sprintf(`다음 SQL 쿼리를 한국어로 설명해주세요:

%s

설명:`, query)

	return g.generate(ctx, prompt)
}

func (g *GroqProvider) ValidateQuery(ctx context.Context, query string, schema *models.Schema) (*models.QueryValidation, error) {
	prompt := buildValidatePrompt(query, schema)

	start := time.Now()
	response, err := g.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()

	validation := parseValidationResponse(response, query)
	validation.AIResponseTime = elapsed

	return validation, nil
}

