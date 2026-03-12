# ─── Docs generation ──────────────────────────────────────────────────────────
#
# make docs          Regenerate all documentation (frontend + backend).
# make docs-serve    Serve docs/ locally at http://localhost:4000
# make install-tools Install gomarkdoc and TypeDoc devDependencies (run once).

GOPATH_BIN   := $(shell go env GOPATH)/bin
GOMARKDOC    := $(GOPATH_BIN)/gomarkdoc

DOCS_DIR     := docs
DOCS_BACKEND := $(DOCS_DIR)/backend
DOCS_FRONTEND:= $(DOCS_DIR)/frontend
DOCS_PORT    := 4000

# Internal Go packages to document (integration is test-only, excluded).
GO_INTERNAL_PACKAGES := \
	./internal/ai \
	./internal/config \
	./internal/crashreport \
	./internal/ddl \
	./internal/filesystem \
	./internal/gitrepo \
	./internal/logger \
	./internal/sfconfig \
	./internal/snowflake \
	./internal/telemetry

.PHONY: docs docs-frontend docs-backend docs-serve install-tools

## docs: Regenerate all documentation (frontend + backend).
docs: docs-frontend docs-backend
	@echo ""
	@echo "Docs written to $(DOCS_DIR)/."
	@echo "Run 'make docs-serve' to view at http://localhost:$(DOCS_PORT)"

## docs-frontend: Generate TypeScript/React docs with TypeDoc → docs/frontend/
docs-frontend:
	@echo "==> Frontend docs (TypeDoc)…"
	@mkdir -p $(DOCS_FRONTEND)
	cd frontend && npx typedoc --options typedoc.json

## docs-backend: Generate Go docs with gomarkdoc → docs/backend/
docs-backend:
	@echo "==> Backend docs (gomarkdoc)…"
	@mkdir -p $(DOCS_BACKEND)/internal
	$(GOMARKDOC) --output "$(DOCS_BACKEND)/app.md" .
	$(GOMARKDOC) --output "$(DOCS_BACKEND)/{{.Dir}}.md" $(GO_INTERNAL_PACKAGES)

## docs-serve: Serve docs/ locally at http://localhost:$(DOCS_PORT)
docs-serve:
	@echo "Serving docs at http://localhost:$(DOCS_PORT) — Ctrl+C to stop"
	python3 -m http.server $(DOCS_PORT) --directory $(DOCS_DIR)

## install-tools: Install gomarkdoc and TypeDoc (run once per machine).
install-tools:
	@echo "==> Installing gomarkdoc…"
	go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest
	@echo "==> Installing TypeDoc…"
	cd frontend && npm install --save-dev typedoc typedoc-plugin-markdown
	@echo "Done. You can now run 'make docs'."
