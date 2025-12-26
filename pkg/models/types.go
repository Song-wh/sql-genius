package models

// DBType 지원하는 데이터베이스 종류
type DBType string

const (
	MySQL      DBType = "mysql"
	PostgreSQL DBType = "postgresql"
	Oracle     DBType = "oracle"
	SQLServer  DBType = "sqlserver"
)

// AIProvider AI 제공자 종류
type AIProvider string

const (
	Ollama AIProvider = "ollama"
	Groq   AIProvider = "groq"
)

// DBConfig 데이터베이스 연결 설정
type DBConfig struct {
	Type     DBType `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// Table 테이블 정보
type Table struct {
	Name        string   `json:"name"`
	Columns     []Column `json:"columns"`
	PrimaryKey  []string `json:"primary_key"`
	ForeignKeys []FK     `json:"foreign_keys"`
	Indexes     []Index  `json:"indexes"`
}

// Column 컬럼 정보
type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default,omitempty"`
	Comment    string `json:"comment,omitempty"`
	IsPK       bool   `json:"is_pk"`
	IsFK       bool   `json:"is_fk"`
	IsUnique   bool   `json:"is_unique"`
	IsAutoIncr bool   `json:"is_auto_incr"`
}

// FK 외래키 정보
type FK struct {
	Name            string `json:"name"`
	Column          string `json:"column"`
	RefTable        string `json:"ref_table"`
	RefColumn       string `json:"ref_column"`
	OnDelete        string `json:"on_delete,omitempty"`
	OnUpdate        string `json:"on_update,omitempty"`
}

// Index 인덱스 정보
type Index struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	IsUnique bool     `json:"is_unique"`
	Type     string   `json:"type"` // BTREE, HASH, FULLTEXT 등
}

// Schema 전체 스키마 정보
type Schema struct {
	Database string  `json:"database"`
	Tables   []Table `json:"tables"`
	DBType   DBType  `json:"db_type"`
}

// QueryRequest 쿼리 생성 요청
type QueryRequest struct {
	Prompt     string `json:"prompt"`      // 자연어 요청
	Schema     Schema `json:"schema"`      // 스키마 정보
	QueryType  string `json:"query_type"`  // SELECT, INSERT, UPDATE, DELETE, ALTER
	Optimize   bool   `json:"optimize"`    // 최적화 여부
}

// QueryResponse 쿼리 생성 응답
type QueryResponse struct {
	Query       string   `json:"query"`        // 생성된 SQL 쿼리
	Explanation string   `json:"explanation"`  // 쿼리 설명
	Tips        []string `json:"tips"`         // 최적화 팁
	ExecuteTime int64    `json:"execute_time"` // 예상 실행 시간 (ms)
}

// AIConfig AI 설정
type AIConfig struct {
	Provider AIProvider `json:"provider"`
	Model    string     `json:"model"`
	Endpoint string     `json:"endpoint"` // Ollama: http://localhost:11434, Groq: https://api.groq.com
	APIKey   string     `json:"api_key,omitempty"`
}

// QueryValidation 쿼리 검증 결과
type QueryValidation struct {
	IsValid         bool     `json:"is_valid"`          // 문법 유효 여부
	Score           int      `json:"score"`             // 성능 점수 (0-100)
	OriginalQuery   string   `json:"original_query"`    // 원본 쿼리
	OptimizedQuery  string   `json:"optimized_query"`   // 최적화된 쿼리
	Issues          []Issue  `json:"issues"`            // 발견된 문제점
	Suggestions     []string `json:"suggestions"`       // 개선 제안
	IndexUsage      []string `json:"index_usage"`       // 사용 가능한 인덱스
	ExecutionPlan   string   `json:"execution_plan"`    // 예상 실행 계획
	EstimatedTime   string   `json:"estimated_time"`    // 예상 실행 시간
	AIResponseTime  int64    `json:"ai_response_time"`  // AI 응답 시간 (ms)
}

// Issue 쿼리 문제점
type Issue struct {
	Type        string `json:"type"`        // error, warning, info
	Message     string `json:"message"`     // 문제 설명
	Location    string `json:"location"`    // 위치 (컬럼, 테이블 등)
	Suggestion  string `json:"suggestion"`  // 해결 방안
}

