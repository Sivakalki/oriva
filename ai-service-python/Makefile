.PHONY: sync lock run dev test test-integration lint fmt typecheck check

sync:
	uv sync

lock:
	uv lock

run:
	uv run oriva-ai

dev:
	uv run uvicorn oriva_ai.app:create_app --factory --reload --host 0.0.0.0 --port 8090

test:
	uv run pytest

# needs a running backend-go MCP server; set ORIVA_GO_MCP_URL (+ optionally
# ORIVA_GO_MCP_TOKEN, ORIVA_GO_MCP_SESSION_ID)
test-integration:
	uv run pytest -m integration

lint:
	uv run ruff check .

fmt:
	uv run ruff format .

typecheck:
	uv run mypy src

check: lint typecheck test
