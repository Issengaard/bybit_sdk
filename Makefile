# Tool versions
GOLANGCI_VERSION ?= v2.7.2

# Directories
TEMP_DIR := .temp
BUILD_DIR := .build

# Test runner (pretty progress/table wrapper around `go test -json`).
TEST_RUNNER := dev/tools/bin/test-runner
TEST_RUNNER_SRC := .claude/scripts/src/test-runner/main.go

# Unit tests live in the module root package and ./testhelper. The integration
# suites под ./integrationtest и ./integrationtest-ws закрыты build-тегами,
# поэтому `go test ./...` собирает только unit-тесты — то, что нам и нужно.
UNIT_PKGS := ./...

# Каждый интеграционный пакет защищён уникальным build-тегом. Передаём весь
# список тегов разом, чтобы собрать и запустить все интеграционные пакеты.
INTEGRATION_TAGS := \
	integrationtestderivativecontract \
	integrationtestderivativeunifiedmargin \
	integrationtestfutureinversefuture \
	integrationtestfutureinverseperpetual \
	integrationtestfutureusdtperpetual \
	integrationtestspotv1 \
	integrationtestv5account \
	integrationtestv5asset \
	integrationtestv5execution \
	integrationtestv5market \
	integrationtestv5order \
	integrationtestv5position \
	integrationtestv5user \
	integrationtestwsspotv1 \
	integrationtestwsv5

# Include dev tools (golangci-lint — см. dev/tools/tools.mk)
-include dev/tools/tools.mk

# Пересобрать раннер, если исходник новее бинарника.
$(TEST_RUNNER): $(TEST_RUNNER_SRC)
	@echo "Building test-runner..."
	@go build -o $(TEST_RUNNER) $(TEST_RUNNER_SRC)

##@ Testing - Тестирование
test: test-unit ## Запустить unit тесты (безопасный дефолт, без обращения к API)

test-unit: $(TEST_RUNNER) ## Запустить unit тесты
	@$(TEST_RUNNER) -title "Unit Tests" $(UNIT_PKGS)

test-race: $(TEST_RUNNER) ## Запустить unit тесты с race detector
	@CGO_ENABLED=1 $(TEST_RUNNER) -race -title "Unit Tests (Race)" $(UNIT_PKGS)

test-verbose test-unit-verbose: $(TEST_RUNNER) ## Запустить unit тесты с полным выводом
	@$(TEST_RUNNER) -v -title "Unit Tests (verbose)" $(UNIT_PKGS)

# ВНИМАНИЕ: интеграционные тесты ходят в реальный Bybit API и требуют ключей
# в окружении (client.WithAuthFromEnv()). Запускать осознанно.
test-integration: $(TEST_RUNNER) ## Запустить интеграционные тесты (нужны API-ключи в env)
	@$(TEST_RUNNER) -integration -tags "$(INTEGRATION_TAGS)" -title "Integration Tests (REST)" ./integrationtest/...
	@$(TEST_RUNNER) -integration -tags "$(INTEGRATION_TAGS)" -title "Integration Tests (WS)" ./integrationtest-ws/...

test-integration-verbose: $(TEST_RUNNER) ## Запустить интеграционные тесты с полным выводом
	@$(TEST_RUNNER) -v -integration -tags "$(INTEGRATION_TAGS)" -title "Integration Tests (REST, verbose)" ./integrationtest/...
	@$(TEST_RUNNER) -v -integration -tags "$(INTEGRATION_TAGS)" -title "Integration Tests (WS, verbose)" ./integrationtest-ws/...

bench: ## Запустить бенчмарки
	go test -bench=. -benchmem $(UNIT_PKGS)

test-coverage: $(TEST_RUNNER) ## Запустить unit тесты с покрытием
	@mkdir -p $(TEMP_DIR)
	@$(TEST_RUNNER) -coverprofile $(TEMP_DIR)/coverage.out -title "Coverage" $(UNIT_PKGS)
	@go tool cover -html=$(TEMP_DIR)/coverage.out -o $(TEMP_DIR)/coverage.html 2>/dev/null || true
	@echo "HTML отчёт: $(TEMP_DIR)/coverage.html"

test-clean: ## Очистить временные файлы тестов
	@rm -rf $(TEMP_DIR)
	@go clean -testcache
	@echo "Временные файлы тестов очищены"

##@ Code Quality - Качество кода

fmt: ## Отформатировать код
	go fmt ./...

vet: ## Запустить go vet
	go vet ./...

lint: ## Запустить линтер
	@if [ -x "$(golangci-lint)" ]; then \
		$(golangci-lint) run ./... --timeout=5m; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... --timeout=5m; \
	else \
		echo "golangci-lint not found. Run 'make tools-setup' to install tools."; \
		exit 1; \
	fi

lint-fix: ## Запустить линтер с автоисправлением
	@if [ -x "$(golangci-lint)" ]; then \
		$(golangci-lint) run ./... --timeout=5m --fix; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... --timeout=5m --fix; \
	else \
		echo "golangci-lint not found. Run 'make tools-setup' to install tools."; \
		exit 1; \
	fi

##@ Audit - Аудит

audit: vet lint test ## Запустить все проверки качества
	go mod verify
	go mod tidy -diff

tidy: ## Привести зависимости в порядок
	go mod tidy

##@ Build - Сборка

clean: ## Очистить артефакты сборки
	@rm -rf $(TEMP_DIR) $(BUILD_DIR) bin/
	@echo "Артефакты сборки очищены"

##@ Release - Релизы

release: ## Создать новый релиз (интерактивно)
	@chmod +x dev/tools/release.sh
	@./dev/tools/release.sh

release-dry-run: ## Предпросмотр релиза без изменений
	@chmod +x dev/tools/release.sh
	@./dev/tools/release.sh --dry-run

release-with-notes: ## Создать релиз с AI-генерированными заметками
	@chmod +x dev/tools/release.sh
	@./dev/tools/release.sh --generate-notes
##@ Help - Справка

help: ## Показать справку
	@echo ""
	@printf '\033[36m╔═══════════════════════════════════════════════════════════════╗\033[0m\n'
	@printf '\033[36m║  %-61s║\033[0m\n' "bybit_sdk - Makefile commands"
	@printf '\033[36m╚═══════════════════════════════════════════════════════════════╝\033[0m\n'
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[0m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[33m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo ""

.PHONY: test test-unit test-race test-verbose test-unit-verbose \
	test-integration test-integration-verbose bench test-coverage test-clean \
	fmt vet lint lint-fix audit tidy clean \
	release release-dry-run release-with-notes help

.DEFAULT_GOAL := help
