"""Entry point: initializes DB and runs the agent loop."""
import logging
import os
import time

from dotenv import load_dotenv

load_dotenv("/app/.env")

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s — %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
log = logging.getLogger(__name__)

import db
import executor
from agent import run_cycle

INTERVAL_MINUTES = int(os.environ.get("CYCLE_INTERVAL_MINUTES", "30"))


def wait_for_market_api(retries: int = 20, delay: int = 5):
    log.info("Waiting for market-api to be available...")
    for i in range(retries):
        if executor.health_check():
            log.info("market-api is ready")
            return
        log.info(f"market-api not ready yet ({i+1}/{retries}), retrying in {delay}s...")
        time.sleep(delay)
    raise RuntimeError("market-api did not become available in time")


def main():
    log.info("=== Trading Agent starting ===")
    log.info(f"Mode: {'DRY RUN' if os.environ.get('DRY_RUN','false')=='true' else 'LIVE'}")
    log.info(f"Model: {os.environ.get('LLM_MODEL', 'qwen2.5:14b')}")
    log.info(f"Cycle interval: {INTERVAL_MINUTES} minutes")

    db.init_db()
    wait_for_market_api()

    while True:
        try:
            run_cycle()
        except Exception as e:
            log.exception(f"Unhandled error in cycle: {e}")

        log.info(f"Sleeping {INTERVAL_MINUTES} minutes until next cycle...")
        time.sleep(INTERVAL_MINUTES * 60)


if __name__ == "__main__":
    main()
