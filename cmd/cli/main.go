package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sql-genius/internal/ai"
	"sql-genius/internal/db"
	"sql-genius/internal/query"
	"sql-genius/internal/schema"
	"sql-genius/pkg/models"
	"strings"
	"time"
)

var (
	// DB 연결 옵션
	dbType   = flag.String("db", "", "데이터베이스 타입 (mysql, postgresql, oracle, sqlserver)")
	dbHost   = flag.String("host", "localhost", "데이터베이스 호스트")
	dbPort   = flag.Int("port", 0, "데이터베이스 포트")
	dbUser   = flag.String("user", "", "데이터베이스 사용자")
	dbPass   = flag.String("password", "", "데이터베이스 비밀번호")
	dbName   = flag.String("database", "", "데이터베이스 이름")

	// 스키마 입력 옵션
	schemaFile = flag.String("schema", "", "스키마 파일 경로 (JSON 또는 DDL)")
	schemaDDL  = flag.String("ddl", "", "DDL 문자열")

	// AI 옵션
	aiProvider  = flag.String("ai", "ollama", "AI 제공자 (ollama, groq)")
	aiModel     = flag.String("model", "", "AI 모델 이름")
	aiEndpoint  = flag.String("endpoint", "", "AI 엔드포인트")
	groqAPIKey  = flag.String("groq-key", "", "Groq API 키 (환경변수 GROQ_API_KEY도 가능)")

	// 기타
	interactive = flag.Bool("i", false, "대화형 모드")
	promptText  = flag.String("prompt", "", "쿼리 생성 프롬프트")
	queryType   = flag.String("type", "SELECT", "쿼리 타입 (SELECT, INSERT, UPDATE, DELETE, ALTER)")
)

const banner = `
╔═══════════════════════════════════════════════════════════╗
║                    🚀 SQL Genius                          ║
║           AI 기반 SQL 쿼리 생성 및 최적화                  ║
╚═══════════════════════════════════════════════════════════╝
`

func main() {
	flag.Parse()

	fmt.Print(banner)

	ctx := context.Background()

	// 스키마 로드
	dbSchema, err := loadSchema(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 스키마 로드 실패: %v\n", err)
		os.Exit(1)
	}

	if dbSchema == nil {
		fmt.Println("💡 사용법:")
		fmt.Println("  1. DB 직접 연결: sql-genius -db mysql -host localhost -port 3306 -user root -password xxx -database mydb")
		fmt.Println("  2. 스키마 파일: sql-genius -schema schema.json")
		fmt.Println("  3. DDL 입력: sql-genius -ddl \"CREATE TABLE ...\"")
		os.Exit(0)
	}

	// AI 제공자 설정
	aiConfig := models.AIConfig{
		Provider: models.AIProvider(*aiProvider),
		Model:    *aiModel,
		Endpoint: *aiEndpoint,
		APIKey:   getAPIKey(),
	}

	provider, err := ai.NewProvider(aiConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ AI 제공자 초기화 실패: %v\n", err)
		os.Exit(1)
	}

	// 연결 상태 확인
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if provider.IsAvailable(checkCtx) {
		fmt.Printf("✅ AI 제공자 연결됨: %s\n", provider.Name())
	} else {
		fmt.Printf("⚠️  AI 제공자 연결 실패 (계속 진행...): %s\n", provider.Name())
	}

	// 쿼리 생성기 초기화
	gen := query.NewGenerator(provider, dbSchema)

	fmt.Printf("📊 로드된 테이블: %d개\n", len(dbSchema.Tables))
	for _, t := range dbSchema.Tables {
		fmt.Printf("   - %s (%d 컬럼)\n", t.Name, len(t.Columns))
	}
	fmt.Println()

	if *interactive || *promptText == "" {
		runInteractive(ctx, gen)
	} else {
		runSingle(ctx, gen)
	}
}

func loadSchema(ctx context.Context) (*models.Schema, error) {
	parser := schema.NewParser()

	// 1. DB 직접 연결
	if *dbType != "" {
		config := models.DBConfig{
			Type:     models.DBType(*dbType),
			Host:     *dbHost,
			Port:     getPort(),
			User:     *dbUser,
			Password: *dbPass,
			Database: *dbName,
		}

		connector, err := db.NewConnector(config)
		if err != nil {
			return nil, err
		}

		if err := connector.Connect(ctx); err != nil {
			return nil, err
		}
		defer connector.Close()

		fmt.Println("✅ 데이터베이스 연결됨")
		return connector.ExtractSchema(ctx)
	}

	// 2. 스키마 파일
	if *schemaFile != "" {
		data, err := os.ReadFile(*schemaFile)
		if err != nil {
			return nil, err
		}

		// JSON 또는 DDL 감지
		if strings.HasSuffix(*schemaFile, ".json") {
			return parser.ParseJSON(data)
		}
		return parser.ParseDDL(string(data), models.MySQL)
	}

	// 3. DDL 문자열
	if *schemaDDL != "" {
		return parser.ParseDDL(*schemaDDL, models.DBType(*dbType))
	}

	return nil, nil
}

func getPort() int {
	if *dbPort != 0 {
		return *dbPort
	}

	// 기본 포트
	switch *dbType {
	case "mysql":
		return 3306
	case "postgresql":
		return 5432
	case "oracle":
		return 1521
	case "sqlserver":
		return 1433
	default:
		return 0
	}
}

func getAPIKey() string {
	if *groqAPIKey != "" {
		return *groqAPIKey
	}
	return os.Getenv("GROQ_API_KEY")
}

func runInteractive(ctx context.Context, gen *query.Generator) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🎯 대화형 모드 시작 (종료: exit 또는 quit)")
	fmt.Println("💡 명령어:")
	fmt.Println("   /select, /insert, /update, /delete, /alter - 쿼리 타입 설정")
	fmt.Println("   /optimize <쿼리> - 쿼리 최적화")
	fmt.Println("   /explain <쿼리> - 쿼리 설명")
	fmt.Println("   /schema - 스키마 정보 출력")
	fmt.Println()

	currentType := "SELECT"

	for {
		fmt.Printf("[%s] > ", currentType)
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// 종료 명령
		if input == "exit" || input == "quit" {
			fmt.Println("👋 종료합니다!")
			break
		}

		// 명령어 처리
		if strings.HasPrefix(input, "/") {
			handleCommand(ctx, gen, input, &currentType)
			continue
		}

		// 쿼리 생성
		fmt.Println("🔄 쿼리 생성 중...")
		start := time.Now()

		resp, err := gen.Generate(ctx, input, currentType)
		if err != nil {
			fmt.Printf("❌ 오류: %v\n\n", err)
			continue
		}

		elapsed := time.Since(start)

		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("📝 생성된 쿼리:")
		fmt.Println(formatSQL(resp.Query))
		fmt.Println()

		if resp.Explanation != "" {
			fmt.Println("💡 설명:")
			fmt.Println("   " + resp.Explanation)
			fmt.Println()
		}

		if len(resp.Tips) > 0 {
			fmt.Println("🚀 최적화 팁:")
			for _, tip := range resp.Tips {
				fmt.Println("   • " + tip)
			}
			fmt.Println()
		}

		fmt.Printf("⏱️  생성 시간: %v (AI 처리: %dms)\n", elapsed, resp.ExecuteTime)
		fmt.Println(strings.Repeat("─", 60))
		fmt.Println()
	}
}

func handleCommand(ctx context.Context, gen *query.Generator, cmd string, currentType *string) {
	parts := strings.SplitN(cmd, " ", 2)
	command := strings.ToLower(parts[0])

	switch command {
	case "/select":
		*currentType = "SELECT"
		fmt.Println("✅ SELECT 모드로 전환")
	case "/insert":
		*currentType = "INSERT"
		fmt.Println("✅ INSERT 모드로 전환")
	case "/update":
		*currentType = "UPDATE"
		fmt.Println("✅ UPDATE 모드로 전환")
	case "/delete":
		*currentType = "DELETE"
		fmt.Println("✅ DELETE 모드로 전환")
	case "/alter":
		*currentType = "ALTER"
		fmt.Println("✅ ALTER 모드로 전환")
	case "/create":
		*currentType = "CREATE"
		fmt.Println("✅ CREATE 모드로 전환")
	case "/optimize":
		if len(parts) < 2 {
			fmt.Println("❌ 사용법: /optimize <쿼리>")
			return
		}
		resp, err := gen.Optimize(ctx, parts[1])
		if err != nil {
			fmt.Printf("❌ 오류: %v\n", err)
			return
		}
		fmt.Println("\n📝 최적화된 쿼리:")
		fmt.Println(formatSQL(resp.Query))
		if len(resp.Tips) > 0 {
			fmt.Println("\n🚀 변경 사항:")
			for _, tip := range resp.Tips {
				fmt.Println("   • " + tip)
			}
		}
	case "/explain":
		if len(parts) < 2 {
			fmt.Println("❌ 사용법: /explain <쿼리>")
			return
		}
		explanation, err := gen.Explain(ctx, parts[1])
		if err != nil {
			fmt.Printf("❌ 오류: %v\n", err)
			return
		}
		fmt.Println("\n💡 쿼리 설명:")
		fmt.Println(explanation)
	case "/schema":
		printSchema(gen.GetSchema())
	default:
		fmt.Println("❌ 알 수 없는 명령어:", command)
	}
	fmt.Println()
}

func runSingle(ctx context.Context, gen *query.Generator) {
	resp, err := gen.Generate(ctx, *promptText, *queryType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 오류: %v\n", err)
		os.Exit(1)
	}

	// JSON 출력
	output, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(output))
}

func printSchema(s *models.Schema) {
	fmt.Printf("\n📊 데이터베이스: %s (%s)\n", s.Database, s.DBType)
	fmt.Println(strings.Repeat("─", 50))
	for _, table := range s.Tables {
		fmt.Printf("\n📋 테이블: %s\n", table.Name)
		for _, col := range table.Columns {
			flags := ""
			if col.IsPK {
				flags += " 🔑"
			}
			if col.IsFK {
				flags += " 🔗"
			}
			if col.IsUnique {
				flags += " ⭐"
			}
			nullable := "NULL"
			if !col.Nullable {
				nullable = "NOT NULL"
			}
			fmt.Printf("   ├─ %s %s %s%s\n", col.Name, col.Type, nullable, flags)
		}
		if len(table.Indexes) > 0 {
			fmt.Println("   └─ 인덱스:")
			for _, idx := range table.Indexes {
				unique := ""
				if idx.IsUnique {
					unique = " (UNIQUE)"
				}
				fmt.Printf("      • %s (%s)%s\n", idx.Name, strings.Join(idx.Columns, ", "), unique)
			}
		}
	}
}

func formatSQL(sql string) string {
	// 간단한 SQL 포맷팅
	keywords := []string{"SELECT", "FROM", "WHERE", "JOIN", "LEFT JOIN", "RIGHT JOIN",
		"INNER JOIN", "ORDER BY", "GROUP BY", "HAVING", "LIMIT", "OFFSET",
		"INSERT INTO", "VALUES", "UPDATE", "SET", "DELETE FROM",
		"CREATE TABLE", "ALTER TABLE", "DROP TABLE", "CREATE INDEX"}

	formatted := sql
	for _, kw := range keywords {
		formatted = strings.ReplaceAll(formatted, " "+kw+" ", "\n"+kw+" ")
		formatted = strings.ReplaceAll(formatted, " "+strings.ToLower(kw)+" ", "\n"+strings.ToLower(kw)+" ")
	}

	// 들여쓰기 추가
	lines := strings.Split(formatted, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, "   "+line)
		}
	}
	return strings.Join(result, "\n")
}

