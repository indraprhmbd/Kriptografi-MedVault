"""Unified Desktop Application Orchestrator for MedVault.

Launches the local FastAPI REST backend and the Electron standalone desktop
application simultaneously, with synchronized lifecycle and clean shutdown.
"""

from pathlib import Path
import subprocess
import sys
import time
import urllib.request
import urllib.error

PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from backend.config import settings
from backend.database import init_database


def is_backend_ready(url: str = "http://127.0.0.1:8000/api/v1/health") -> bool:
    """Probe the backend health endpoint."""
    try:
        with urllib.request.urlopen(url, timeout=1) as response:
            return response.status == 200
    except (urllib.error.URLError, TimeoutError, OSError):
        return False


def main() -> None:
    print("=" * 70)
    print(" MedVault Crypto - Sistem Rekam Medis Kriptografis")
    print(" Mode: Desktop Standalone (Electron + FastAPI)")
    print("=" * 70)

    # 1. Ensure database and storage directories are ready
    print("\n[1/3] Menyiapkan database SQLite dan direktori penyimpanan...")
    init_database()
    print("[OK] Database dan Genesis block siap.")

    # 2. Start FastAPI Backend Subprocess
    print(f"\n[2/3] Memulai server backend FastAPI pada http://{settings.MEDVAULT_API_HOST}:{settings.MEDVAULT_API_PORT}...")
    venv_python = PROJECT_ROOT / ".venv" / "Scripts" / "python.exe"
    python_bin = str(venv_python) if venv_python.is_file() else sys.executable

    backend_cmd = [
        python_bin,
        "-m",
        "uvicorn",
        "backend.main:app",
        "--host",
        settings.MEDVAULT_API_HOST,
        "--port",
        str(settings.MEDVAULT_API_PORT),
    ]

    log_file = PROJECT_ROOT / "storage" / "backend.log"
    log_file.parent.mkdir(parents=True, exist_ok=True)
    backend_log_handle = open(log_file, "a", encoding="utf-8")

    backend_proc = subprocess.Popen(
        backend_cmd,
        cwd=str(PROJECT_ROOT),
        stdout=backend_log_handle,
        stderr=subprocess.STDOUT,
        text=True,
    )

    # Wait for backend health check to respond OK
    print("[WAIT] Menunggu inisialisasi backend API...")
    max_wait_seconds = 15
    start_time = time.time()
    backend_online = False

    while time.time() - start_time < max_wait_seconds:
        if is_backend_ready():
            backend_online = True
            break
        if backend_proc.poll() is not None:
            backend_log_handle.flush()
            recent_logs = log_file.read_text(encoding="utf-8", errors="replace").splitlines()[-20:]
            print(f"[ERROR] Backend gagal berjalan:\n" + "\n".join(recent_logs))
            sys.exit(1)
        time.sleep(0.5)

    if not backend_online:
        print("[ERROR] Timeout: Backend FastAPI tidak merespons dalam 15 detik.")
        backend_proc.terminate()
        backend_log_handle.close()
        sys.exit(1)

    print(f"[OK] Backend FastAPI aktif. Observabilitas log tersimpan di: {log_file}")

    # 3. Ensure Frontend Desktop is up to date
    frontend_dir = PROJECT_ROOT / "frontend_desktop"
    dist_index = frontend_dir / "dist" / "index.html"
    src_dir = frontend_dir / "src"

    needs_rebuild = not dist_index.is_file()
    if not needs_rebuild and src_dir.is_dir():
        dist_mtime = dist_index.stat().st_mtime
        for p in src_dir.rglob("*"):
            if p.is_file() and p.stat().st_mtime > dist_mtime:
                needs_rebuild = True
                break

    if needs_rebuild:
        print("\n[INFO] Membangun bundle frontend desktop terbaru (Vite + React)...")
        build_res = subprocess.run(
            ["pnpm.cmd", "run", "build"],
            cwd=str(frontend_dir),
            shell=True,
        )
        if build_res.returncode != 0:
            print("[WARN] Gagal build via pnpm, mencoba npx vite build...")
            subprocess.run(["npx.cmd", "vite", "build"], cwd=str(frontend_dir), shell=True)
    else:
        print("[OK] Bundle frontend desktop sudah mutakhir.")

    # 4. Launch Electron Desktop Frontend
    print("\n[3/3] Membuka jendela desktop Electron...")

    # Use npx electron or pnpm electron
    electron_cmd = ["npx.cmd", "electron", "."]

    try:
        electron_proc = subprocess.Popen(
            electron_cmd,
            cwd=str(frontend_dir),
            shell=True,
        )
        print("[OK] Aplikasi desktop berjalan. Menunggu penutupan jendela...")
        electron_proc.wait()
    except KeyboardInterrupt:
        print("\n[INFO] Menghentikan aplikasi atas permintaan pengguna...")
    finally:
        print("\n[SHUTDOWN] Menutup proses backend FastAPI...")
        backend_proc.terminate()
        try:
            backend_proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            backend_proc.kill()
        backend_log_handle.close()
        print("[OK] Semua proses MedVault berhasil dihentikan dengan aman.")


if __name__ == "__main__":
    main()
