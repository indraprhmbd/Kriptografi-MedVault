"""FastAPI main application entrypoint for MedVault Crypto.

Configures CORS for desktop frontend communication, registers API routers,
and provides system health check endpoints and request observability logging.
"""

import logging
from logging.handlers import RotatingFileHandler
from pathlib import Path
import sys
import time
import traceback
import uuid

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from backend.config import PROJECT_ROOT, settings
from backend.database import get_db_connection
from backend.routers.auth_router import router as auth_router
from backend.routers.patient_router import router as patient_router
from backend.routers.record_router import router as record_router
from backend.routers.vault_router import router as vault_router
from backend.routers.stego_router import router as stego_router
from backend.routers.audit_router import router as audit_router

# =============================================================================
# Observability Logging Setup
# =============================================================================

LOG_FILE = PROJECT_ROOT / "storage" / "backend.log"
LOG_FILE.parent.mkdir(parents=True, exist_ok=True)

logger = logging.getLogger("medvault")
logger.setLevel(logging.INFO)

formatter = logging.Formatter(
    "[%(asctime)s] [%(levelname)s] [%(name)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)

# File Handler (rotating at 5MB, keep 3 backups)
try:
    file_handler = RotatingFileHandler(
        str(LOG_FILE), maxBytes=5 * 1024 * 1024, backupCount=3, encoding="utf-8"
    )
    file_handler.setFormatter(formatter)
    logger.addHandler(file_handler)
except Exception:
    # Fall back to console logger if log file cannot be opened (e.g. permission or lock)
    pass

# Console Handler
console_handler = logging.StreamHandler(sys.stdout)
console_handler.setFormatter(formatter)
logger.addHandler(console_handler)

app = FastAPI(
    title="MedVault Crypto API",
    description="Sistem Keamanan Rekam Medis - Kriptografi, Steganografi, dan Audit Blockchain",
    version=settings.app_version,
)

# =============================================================================
# Observability Middleware
# =============================================================================

@app.middleware("http")
async def observability_middleware(request: Request, call_next):
    req_id = uuid.uuid4().hex[:8]
    start_time = time.time()
    client_ip = request.client.host if request.client else "unknown"
    logger.info(f"[{req_id}] INCOMING {request.method} {request.url.path} from {client_ip}")

    try:
        response = await call_next(request)
        duration_ms = (time.time() - start_time) * 1000
        logger.info(
            f"[{req_id}] COMPLETED {request.method} {request.url.path} -> "
            f"HTTP {response.status_code} ({duration_ms:.1f}ms)"
        )
        return response
    except Exception as exc:
        duration_ms = (time.time() - start_time) * 1000
        logger.error(
            f"[{req_id}] FAILED {request.method} {request.url.path} ({duration_ms:.1f}ms): {exc}\n"
            f"{traceback.format_exc()}"
        )
        return JSONResponse(
            status_code=500,
            content={"detail": f"Internal Server Error: {str(exc)}"},
        )

# Enable CORS for desktop frontend and dev servers
app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:5173",
        "http://127.0.0.1:5173",
        "app://.",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Register REST Routers
app.include_router(auth_router)
app.include_router(patient_router)
app.include_router(record_router)
app.include_router(vault_router)
app.include_router(stego_router)
app.include_router(audit_router)


@app.get("/api/v1/system/logs", tags=["System"])
def get_system_logs(lines: int = 100) -> dict:
    """Return the recent lines of the backend log file for live observability."""
    if not LOG_FILE.is_file():
        return {"status": "ok", "lines": [], "message": "Log file belum terbuat"}

    try:
        content = LOG_FILE.read_text(encoding="utf-8", errors="replace")
        all_lines = content.splitlines()
        tail = all_lines[-lines:] if len(all_lines) > lines else all_lines
        return {"status": "ok", "total_lines": len(all_lines), "lines": tail}
    except Exception as exc:
        return {"status": "error", "message": str(exc), "lines": []}


@app.get("/api/v1/health", tags=["System"])
def health_check() -> dict:
    """Verify backend and SQLite database connectivity."""
    db_ok = False
    error_msg = None
    try:
        conn = get_db_connection()
        conn.execute("SELECT 1")
        conn.close()
        db_ok = True
    except Exception as exc:
        error_msg = str(exc)

    return {
        "status": "ok" if db_ok else "error",
        "database_connected": db_ok,
        "detail": error_msg,
        "vigenere_key_configured": bool(settings.MEDVAULT_VIGENERE_KEY),
        "aes128_key_configured": len(settings.aes128_key_bytes) == 16,
        "chacha20_key_configured": len(settings.chacha20_key_bytes) == 32,
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "backend.main:app",
        host=settings.MEDVAULT_API_HOST,
        port=settings.MEDVAULT_API_PORT,
        reload=True,
    )
