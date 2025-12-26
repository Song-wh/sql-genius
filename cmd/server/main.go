package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sql-genius/internal/ai"
	"sql-genius/internal/db"
	"sql-genius/internal/query"
	"sql-genius/internal/schema"
	"sql-genius/pkg/models"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

var (
	port       = flag.Int("port", 8080, "서버 포트")
	aiProvider = flag.String("ai", "ollama", "AI 제공자 (ollama, groq)")
	aiModel    = flag.String("model", "", "AI 모델 이름")
	aiEndpoint = flag.String("endpoint", "", "AI 엔드포인트")
	groqAPIKey = flag.String("groq-key", "", "Groq API 키")
)

type Server struct {
	provider   ai.Provider
	generator  *query.Generator
	parser     *schema.Parser
	dbConn     db.Connector
	schema     *models.Schema
}

type GenerateRequest struct {
	Prompt    string        `json:"prompt"`
	QueryType string        `json:"query_type"`
	Schema    models.Schema `json:"schema,omitempty"`
}

type ConnectRequest struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type SchemaRequest struct {
	DDL    string `json:"ddl,omitempty"`
	JSON   string `json:"json,omitempty"`
	DBType string `json:"db_type"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func main() {
	flag.Parse()

	fmt.Println(`
╔═══════════════════════════════════════════════════════════╗
║                    🚀 SQL Genius Server                   ║
╚═══════════════════════════════════════════════════════════╝`)

	// AI 제공자 초기화
	aiConfig := models.AIConfig{
		Provider: models.AIProvider(*aiProvider),
		Model:    *aiModel,
		Endpoint: *aiEndpoint,
		APIKey:   getAPIKey(),
	}

	provider, err := ai.NewProvider(aiConfig)
	if err != nil {
		log.Fatalf("AI 제공자 초기화 실패: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if provider.IsAvailable(ctx) {
		fmt.Printf("✅ AI 제공자 연결됨: %s\n", provider.Name())
	} else {
		fmt.Printf("⚠️  AI 제공자 연결 대기 중: %s\n", provider.Name())
	}

	server := &Server{
		provider: provider,
		parser:   schema.NewParser(),
	}

	// 라우터 설정
	mux := http.NewServeMux()

	// API 라우트
	mux.HandleFunc("/api/generate", server.handleGenerate)
	mux.HandleFunc("/api/optimize", server.handleOptimize)
	mux.HandleFunc("/api/explain", server.handleExplain)
	mux.HandleFunc("/api/validate", server.handleValidate)
	mux.HandleFunc("/api/connect", server.handleConnect)
	mux.HandleFunc("/api/disconnect", server.handleDisconnect)
	mux.HandleFunc("/api/schema/parse", server.handleParseDDL)
	mux.HandleFunc("/api/schema/current", server.handleGetSchema)
	mux.HandleFunc("/api/schema/export", server.handleExportSchema)
	mux.HandleFunc("/api/schema/table", server.handleTableDetail)
	mux.HandleFunc("/api/schema/sample", server.handleSampleData)
	mux.HandleFunc("/api/execute", server.handleExecute)
	mux.HandleFunc("/api/status", server.handleStatus)

	// 정적 파일 서빙
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// CORS 미들웨어
	handler := corsMiddleware(mux)

	fmt.Printf("🌐 서버 시작: http://localhost:%d\n", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), handler))
}

func getAPIKey() string {
	if *groqAPIKey != "" {
		return *groqAPIKey
	}
	return os.Getenv("GROQ_API_KEY")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: data})
}

func (s *Server) jsonError(w http.ResponseWriter, err string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: false, Error: err})
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 스키마 설정
	var targetSchema *models.Schema
	if len(req.Schema.Tables) > 0 {
		targetSchema = &req.Schema
	} else if s.schema != nil {
		targetSchema = s.schema
	} else {
		s.jsonError(w, "스키마가 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	gen := query.NewGenerator(s.provider, targetSchema)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := gen.Generate(ctx, req.Prompt, req.QueryType)
	if err != nil {
		s.jsonError(w, "쿼리 생성 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, resp)
}

func (s *Server) handleOptimize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청", http.StatusBadRequest)
		return
	}

	if s.schema == nil {
		s.jsonError(w, "스키마가 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	gen := query.NewGenerator(s.provider, s.schema)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := gen.Optimize(ctx, req.Query)
	if err != nil {
		s.jsonError(w, "최적화 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, resp)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청", http.StatusBadRequest)
		return
	}

	gen := query.NewGenerator(s.provider, s.schema)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	explanation, err := gen.Explain(ctx, req.Query)
	if err != nil {
		s.jsonError(w, "설명 생성 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"explanation": explanation})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청", http.StatusBadRequest)
		return
	}

	config := models.DBConfig{
		Type:     models.DBType(req.Type),
		Host:     req.Host,
		Port:     req.Port,
		User:     req.User,
		Password: req.Password,
		Database: req.Database,
	}

	conn, err := db.NewConnector(config)
	if err != nil {
		s.jsonError(w, "커넥터 생성 실패: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := conn.Connect(ctx); err != nil {
		s.jsonError(w, "연결 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 스키마 추출
	schema, err := conn.ExtractSchema(ctx)
	if err != nil {
		s.jsonError(w, "스키마 추출 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.dbConn = conn
	s.schema = schema
	s.generator = query.NewGenerator(s.provider, schema)

	s.jsonResponse(w, map[string]interface{}{
		"connected": true,
		"schema":    schema,
	})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.dbConn != nil {
		s.dbConn.Close()
		s.dbConn = nil
	}
	s.schema = nil
	s.generator = nil

	s.jsonResponse(w, map[string]bool{"disconnected": true})
}

func (s *Server) handleParseDDL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	var req SchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청", http.StatusBadRequest)
		return
	}

	var parsedSchema *models.Schema
	var err error

	if req.DDL != "" {
		dbType := models.MySQL
		if req.DBType != "" {
			dbType = models.DBType(req.DBType)
		}
		parsedSchema, err = s.parser.ParseDDL(req.DDL, dbType)
	} else if req.JSON != "" {
		parsedSchema, err = s.parser.ParseJSON([]byte(req.JSON))
	} else {
		s.jsonError(w, "DDL 또는 JSON이 필요합니다", http.StatusBadRequest)
		return
	}

	if err != nil {
		s.jsonError(w, "파싱 실패: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.schema = parsedSchema
	s.generator = query.NewGenerator(s.provider, parsedSchema)

	s.jsonResponse(w, parsedSchema)
}

func (s *Server) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	if s.schema == nil {
		s.jsonError(w, "스키마가 설정되지 않았습니다", http.StatusNotFound)
		return
	}
	s.jsonResponse(w, s.schema)
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	if s.dbConn == nil {
		s.jsonError(w, "데이터베이스에 연결되어 있지 않습니다", http.StatusBadRequest)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := s.dbConn.ExecuteQuery(ctx, req.Query)
	if err != nil {
		s.jsonError(w, "쿼리 실행 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, result)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"ai_provider":   s.provider.Name(),
		"ai_available":  s.provider.IsAvailable(r.Context()),
		"db_connected":  s.dbConn != nil,
		"schema_loaded": s.schema != nil,
	}

	if s.schema != nil {
		status["tables_count"] = len(s.schema.Tables)
		status["db_type"] = s.schema.DBType
	}

	s.jsonResponse(w, status)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, "POST 요청만 허용됩니다", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "잘못된 요청", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		s.jsonError(w, "쿼리가 필요합니다", http.StatusBadRequest)
		return
	}

	if s.schema == nil {
		s.jsonError(w, "스키마가 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	validation, err := s.provider.ValidateQuery(ctx, req.Query, s.schema)
	if err != nil {
		s.jsonError(w, "쿼리 검증 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, validation)
}

func (s *Server) handleExportSchema(w http.ResponseWriter, r *http.Request) {
	if s.schema == nil {
		s.jsonError(w, "스키마가 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	schemaJSON, err := json.MarshalIndent(s.schema, "", "  ")
	if err != nil {
		s.jsonError(w, "스키마 변환 실패", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=schema.json")
	w.Write(schemaJSON)
}

func (s *Server) handleTableDetail(w http.ResponseWriter, r *http.Request) {
	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		s.jsonError(w, "테이블 이름이 필요합니다", http.StatusBadRequest)
		return
	}

	if s.schema == nil {
		s.jsonError(w, "스키마가 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	// 테이블 찾기
	var targetTable *models.Table
	for i := range s.schema.Tables {
		if s.schema.Tables[i].Name == tableName {
			targetTable = &s.schema.Tables[i]
			break
		}
	}

	if targetTable == nil {
		s.jsonError(w, "테이블을 찾을 수 없습니다: "+tableName, http.StatusNotFound)
		return
	}

	s.jsonResponse(w, targetTable)
}

func (s *Server) handleSampleData(w http.ResponseWriter, r *http.Request) {
	tableName := r.URL.Query().Get("table")
	limitStr := r.URL.Query().Get("limit")
	if tableName == "" {
		s.jsonError(w, "테이블 이름이 필요합니다", http.StatusBadRequest)
		return
	}

	if s.dbConn == nil {
		s.jsonError(w, "데이터베이스에 연결되어 있지 않습니다", http.StatusBadRequest)
		return
	}

	limit := 10
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit > 100 {
			limit = 100
		}
	}

	// DB 타입에 따른 쿼리 생성
	var query string
	switch s.dbConn.Type() {
	case models.MySQL, models.PostgreSQL:
		query = fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
	case models.SQLServer:
		query = fmt.Sprintf("SELECT TOP %d * FROM %s", limit, tableName)
	case models.Oracle:
		query = fmt.Sprintf("SELECT * FROM %s WHERE ROWNUM <= %d", tableName, limit)
	default:
		query = fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := s.dbConn.ExecuteQuery(ctx, query)
	if err != nil {
		s.jsonError(w, "샘플 데이터 조회 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"table":   tableName,
		"columns": result.Columns,
		"rows":    result.Rows,
		"count":   len(result.Rows),
	})
}

