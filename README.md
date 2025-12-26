# SQL Genius ⚡

AI 기반 SQL 쿼리 생성 및 최적화 도구

## 특징

- 🚀 **자연어 → SQL 변환**: 자연어로 원하는 쿼리를 설명하면 최적화된 SQL 생성
- 🔄 **쿼리 최적화**: 기존 쿼리를 분석하고 더 빠른 버전 제안
- 📊 **다중 DB 지원**: MySQL, PostgreSQL, Oracle, SQL Server
- 🤖 **무료 AI**: Ollama (로컬) 또는 Groq (클라우드, 무료)
- 🖥️ **CLI & Web UI**: 터미널과 웹 브라우저 모두 지원

## 설치

### 1. 의존성 설치

```bash
cd sql-genius
go mod tidy
```

### 2. AI 설정 (택일)

#### Option A: Ollama (로컬, 무료)
```bash
# Ollama 설치: https://ollama.ai
ollama pull llama3.2
```

#### Option B: Groq (클라우드, 무료)
```bash
# https://console.groq.com 에서 API 키 발급
export GROQ_API_KEY="your-api-key"
```

## 사용법

### CLI 모드

#### 1. DB 직접 연결
```bash
go run ./cmd/cli -db mysql -host localhost -port 3306 -user root -password xxx -database mydb -i
```

#### 2. 스키마 파일 사용
```bash
go run ./cmd/cli -schema schema.json -i
```

#### 3. DDL 직접 입력
```bash
go run ./cmd/cli -ddl "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100))" -i
```

### Web UI 모드

```bash
go run ./cmd/server -port 8080
# 브라우저에서 http://localhost:8080 접속
```

### CLI 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-db` | DB 타입 (mysql, postgresql, oracle, sqlserver) | - |
| `-host` | DB 호스트 | localhost |
| `-port` | DB 포트 | 자동 |
| `-user` | DB 사용자 | - |
| `-password` | DB 비밀번호 | - |
| `-database` | DB 이름 | - |
| `-schema` | 스키마 파일 경로 (JSON/DDL) | - |
| `-ddl` | DDL 문자열 | - |
| `-ai` | AI 제공자 (ollama, groq) | ollama |
| `-model` | AI 모델 | 자동 |
| `-endpoint` | AI 엔드포인트 | 자동 |
| `-groq-key` | Groq API 키 | 환경변수 |
| `-i` | 대화형 모드 | false |
| `-prompt` | 쿼리 생성 프롬프트 | - |
| `-type` | 쿼리 타입 | SELECT |

### CLI 명령어 (대화형 모드)

```
/select     - SELECT 모드
/insert     - INSERT 모드
/update     - UPDATE 모드
/delete     - DELETE 모드
/alter      - ALTER 모드
/create     - CREATE 모드
/optimize <query>  - 쿼리 최적화
/explain <query>   - 쿼리 설명
/schema     - 스키마 정보 출력
exit/quit   - 종료
```

## 예제

### 자연어 쿼리 예시

```
> 최근 30일간 주문량이 많은 상위 10개 제품 조회
> 이메일이 gmail인 사용자 중 주문 이력이 있는 사람 조회
> 카테고리별 평균 주문 금액 조회 (금액 높은 순)
> 6개월 이상 주문이 없는 비활성 사용자 삭제
> users 테이블에 phone 컬럼 추가 (VARCHAR(20), nullable)
```

## 스키마 파일 형식

### JSON 형식
```json
{
  "database": "mydb",
  "db_type": "mysql",
  "tables": [
    {
      "name": "users",
      "columns": [
        {"name": "id", "type": "INT", "is_pk": true, "is_auto_incr": true},
        {"name": "name", "type": "VARCHAR(100)", "nullable": false},
        {"name": "email", "type": "VARCHAR(255)", "is_unique": true}
      ],
      "primary_key": ["id"],
      "indexes": [
        {"name": "idx_email", "columns": ["email"], "is_unique": true}
      ]
    }
  ]
}
```

### DDL 형식
```sql
CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE
);

CREATE INDEX idx_email ON users (email);
```

## 지원 데이터베이스

| DB | 드라이버 | 기본 포트 |
|----|----------|-----------|
| MySQL | github.com/go-sql-driver/mysql | 3306 |
| PostgreSQL | github.com/lib/pq | 5432 |
| Oracle | github.com/sijms/go-ora/v2 | 1521 |
| SQL Server | github.com/denisenkom/go-mssqldb | 1433 |

## AI 모델

### Ollama (로컬)
- llama3.2 (기본)
- codellama
- mistral
- qwen2.5-coder

### Groq (클라우드)
- llama-3.3-70b-versatile (기본, 무료)
- mixtral-8x7b-32768

## 라이선스

MIT License

