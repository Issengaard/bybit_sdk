BASE_PATH := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
TOOLS_DIR := $(BASE_PATH)
TOOLS_BIN := $(TOOLS_DIR)/bin
PATH := $(TOOLS_BIN):$(PATH)
export PATH

##@ Tools - Инструменты разработки

define print-tools-header
@echo ""
@echo "╔══════════════════════════════════════════════════════════════╗"
@echo "║                      Development Tools                       ║"
@echo "╚══════════════════════════════════════════════════════════════╝"
@echo ""
endef

.PHONY: tools-clean
tools-clean: ## Очистить установленные инструменты
	$(print-tools-header)
	@rm -rf $(TOOLS_BIN)
	@echo "Tools cleaned successfully"

# Ensure the tools bin directory exists without ever wiping it (non-destructive).
$(TOOLS_BIN):
	@mkdir -p $(TOOLS_BIN)

golangci-lint=$(TOOLS_BIN)/golangci-lint-$(subst .,-,$(GOLANGCI_VERSION))
$(golangci-lint): | $(TOOLS_BIN)
	@echo "Installing golangci-lint version $(GOLANGCI_VERSION)..."
	@rm -f "$(TOOLS_BIN)/golangci-lint"
	@n=0; max=3; \
	until [ $$n -ge $$max ]; do \
		if curl -sSfL --connect-timeout 30 --max-time 300 https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
			| sh -s -- -b $(TOOLS_BIN) $(GOLANGCI_VERSION); then \
			break; \
		fi; \
		n=$$((n + 1)); \
		if [ $$n -lt $$max ]; then \
			echo "golangci-lint install failed ($$n/$$max attempts), retrying in 3s..."; \
			sleep 3; \
		fi; \
	done; \
	if [ ! -x "$(TOOLS_BIN)/golangci-lint" ]; then \
		echo "ERROR: golangci-lint $(GOLANGCI_VERSION) installation failed after $$max attempts"; \
		exit 1; \
	fi
	mv $(TOOLS_BIN)/golangci-lint ${golangci-lint}

.PHONY: tools-setup
tools-setup: ## Установить все инструменты разработки
	$(print-tools-header)
	@$(MAKE) --no-print-directory _tools-do-setup

.PHONY: _tools-do-setup
_tools-do-setup: | $(golangci-lint)
	@echo "Tools installed successfully"

.PHONY: tools-version
tools-version: ## Показать версии установленных инструментов
	$(print-tools-header)
	@printf "  \033[1m%-22s %-15s %s\033[0m\n" "Tool" "Version" "Status"
	@printf "  %-22s %-15s %s\n" "──────────────────────" "───────────────" "──────────────"
	@printf "  %-22s %-15s " "golangci-lint" "$(GOLANGCI_VERSION)"; \
		if [ -x "$(golangci-lint)" ]; then printf "\033[32m✓ installed\033[0m\n"; else printf "\033[31m✗ not installed\033[0m\n"; fi
	@echo ""

.PHONY: tools-update
tools-update: ## Проверить обновления и установить последние версии инструментов
	$(print-tools-header)
	@echo "Checking for updates..."; \
	echo ""; \
	MAKEFILE="$(realpath $(firstword $(MAKEFILE_LIST)))"; \
	UPDATED=0; \
	cp "$$MAKEFILE" "$$MAKEFILE.bak"; \
	fetch_latest() { \
		local tag; \
		tag=$$(curl -sL "https://api.github.com/repos/$$1/releases/latest" 2>/dev/null \
			| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1); \
		if [ -z "$$tag" ] && command -v gh >/dev/null 2>&1; then \
			tag=$$(gh api "repos/$$1/releases/latest" --jq '.tag_name' 2>/dev/null); \
		fi; \
		echo "$$tag"; \
	}; \
	check_update() { \
		local name="$$1" var="$$2" current="$$3" latest="$$4"; \
		if [ -z "$$latest" ]; then \
			printf "  \033[33m⚠\033[0m  %-20s %-15s → \033[33mfailed to fetch\033[0m\n" "$$name" "$$current"; \
		elif [ "$$current" = "$$latest" ]; then \
			printf "  \033[32m✓\033[0m  %-20s %s \033[32m(up to date)\033[0m\n" "$$name" "$$current"; \
		else \
			printf "  \033[36m↑\033[0m  %-20s %-15s → \033[36m%s\033[0m\n" "$$name" "$$current" "$$latest"; \
			sed -i "s/^$$var ?= .*/$$var ?= $$latest/" "$$MAKEFILE"; \
			UPDATED=1; \
		fi; \
	}; \
	\
	LATEST=$$(fetch_latest "golangci/golangci-lint"); \
	check_update "golangci-lint" "GOLANGCI_VERSION" "$(GOLANGCI_VERSION)" "$$LATEST"; \
	\
	echo ""; \
	if [ "$$UPDATED" = "1" ]; then \
		echo "Versions updated in Makefile. Running tools-setup..."; \
		echo ""; \
		if $(MAKE) tools-setup; then \
			rm -f "$$MAKEFILE.bak"; \
		else \
			echo ""; \
			echo "tools-setup failed — reverting version bumps in Makefile"; \
			mv "$$MAKEFILE.bak" "$$MAKEFILE"; \
			exit 1; \
		fi; \
	else \
		echo "All tools are up to date. No changes needed."; \
		rm -f "$$MAKEFILE.bak"; \
	fi
