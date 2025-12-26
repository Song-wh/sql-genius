.PHONY: all build clean run-cli run-server test deps

# 변수
BINARY_CLI = sql-genius-cli
BINARY_SERVER = sql-genius-server
GO = go

# 기본 타겟
all: deps build

# 의존성 설치
deps:
	$(GO) mod tidy

# 빌드
build: build-cli build-server

build-cli:
	$(GO) build -o bin/$(BINARY_CLI) ./cmd/cli

build-server:
	$(GO) build -o bin/$(BINARY_SERVER) ./cmd/server

# 실행
run-cli:
	$(GO) run ./cmd/cli -i

run-server:
	$(GO) run ./cmd/server -port 8080

# 특정 DB로 CLI 실행
run-mysql:
	$(GO) run ./cmd/cli -db mysql -host localhost -port 3306 -user root -database test -i

run-postgres:
	$(GO) run ./cmd/cli -db postgresql -host localhost -port 5432 -user postgres -database test -i

# Groq으로 실행
run-groq:
	$(GO) run ./cmd/cli -ai groq -i

run-server-groq:
	$(GO) run ./cmd/server -ai groq -port 8080

# 테스트
test:
	$(GO) test -v ./...

# 정리
clean:
	rm -rf bin/
	$(GO) clean

# 크로스 컴파일
build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY_CLI)-linux ./cmd/cli
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY_SERVER)-linux ./cmd/server

build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o bin/$(BINARY_CLI).exe ./cmd/cli
	GOOS=windows GOARCH=amd64 $(GO) build -o bin/$(BINARY_SERVER).exe ./cmd/server

build-mac:
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY_CLI)-mac ./cmd/cli
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY_SERVER)-mac ./cmd/server

# 도움말
help:
	@echo "사용 가능한 타겟:"
	@echo "  make deps        - 의존성 설치"
	@echo "  make build       - CLI와 Server 빌드"
	@echo "  make run-cli     - CLI 대화형 모드 실행"
	@echo "  make run-server  - Web 서버 실행 (포트 8080)"
	@echo "  make run-groq    - Groq AI로 CLI 실행"
	@echo "  make test        - 테스트 실행"
	@echo "  make clean       - 빌드 파일 정리"

