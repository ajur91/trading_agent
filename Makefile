.PHONY: setup build up down logs pull-model status clean

# First time setup
setup:
	@echo "Copying .env.example to .env..."
	@cp -n .env.example .env || echo ".env already exists, skipping"
	@echo ""
	@echo "Edit .env with your OKX credentials, then run: make pull-model && make up"

# Pull the LLM model into Ollama (run once before first start)
pull-model:
	@MODEL=$$(grep LLM_MODEL .env | cut -d= -f2 | tr -d '[:space:]'); \
	echo "Pulling model: $$MODEL"; \
	docker compose run --rm ollama ollama pull $$MODEL

# Build all images
build:
	docker compose build

# Start all services (DRY_RUN=true by default)
up:
	docker compose up -d
	@echo ""
	@echo "Services started. Use 'make logs' to monitor."
	@echo "Agent is in DRY_RUN mode — check .env to change."

# Stop all services
down:
	docker compose down

# Show agent logs
logs:
	docker compose logs -f agent

# Show logs from all services
logs-all:
	docker compose logs -f

# Check service status
status:
	docker compose ps
	@echo ""
	@echo "=== Open positions (from agent DB) ==="
	@docker compose exec agent python -c \
		"import db; db.init_db(); trades=db.get_open_trades(); \
		[print(f'{t[\"symbol\"]} | {t[\"direction\"]} | entry: {t[\"entry_price\"]} | SL: {t[\"sl_price\"]} | TP: {t[\"tp_price\"]}') for t in trades] or print('No open trades')"

# Run a single agent cycle manually (useful for testing)
run-once:
	docker compose exec agent python -c "import db; db.init_db(); from agent import run_cycle; run_cycle()"

# Show last 20 decisions from DB
decisions:
	@docker compose exec agent python -c \
		"import db, json; db.init_db(); \
		conn=db.get_conn(); \
		rows=conn.execute('SELECT timestamp, action, symbol, reasoning FROM decisions ORDER BY id DESC LIMIT 20').fetchall(); \
		[print(f'{r[0]} | {r[1]:15} | {str(r[2]):20} | {str(r[3])[:80]}') for r in rows]"

# Remove all containers and volumes (WARNING: deletes agent DB)
clean:
	docker compose down -v
