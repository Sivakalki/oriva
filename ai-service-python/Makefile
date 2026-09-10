.PHONY: sync lock run dev test lint fmt typecheck check

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

lint:
	uv run ruff check .

fmt:
	uv run ruff format .

typecheck:
	uv run mypy src

check: lint typecheck test
