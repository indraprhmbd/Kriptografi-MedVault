"""Release Packaging Script for MedVault Crypto.

Packages the complete standalone desktop application bundle into a distributable
ZIP archive with SHA-256 checksum verification according to Semantic Versioning.
"""

import hashlib
from pathlib import Path
import shutil
import sys
import zipfile

PROJECT_ROOT = Path(__file__).resolve().parent.parent


def get_version() -> str:
    version_file = PROJECT_ROOT / "VERSION"
    if version_file.is_file():
        return version_file.read_text(encoding="utf-8").strip()
    return "1.0.0"


def compute_sha256(file_path: Path) -> str:
    hasher = hashlib.sha256()
    with open(file_path, "rb") as f:
        while chunk := f.read(65536):
            hasher.update(chunk)
    return hasher.hexdigest()


def copy_tree_filtered(src: Path, dst: Path, ignore_patterns: set[str]) -> None:
    dst.mkdir(parents=True, exist_ok=True)
    for item in src.iterdir():
        if item.name in ignore_patterns:
            continue
        target = dst / item.name
        if item.is_dir():
            copy_tree_filtered(item, target, ignore_patterns)
        else:
            shutil.copy2(item, target)


def main() -> None:
    version = get_version()
    print("=" * 70)
    print(f" MedVault Crypto - Packaging Release v{version}")
    print("=" * 70)

    dist_packages = PROJECT_ROOT / "dist" / "packages"
    release_name = f"MedVault-v{version}-windows-x64"
    staging_dir = dist_packages / release_name
    zip_output = dist_packages / f"{release_name}.zip"
    sha_output = dist_packages / f"{release_name}.zip.sha256"

    # Clean previous packaging artifacts
    if staging_dir.exists():
        shutil.rmtree(staging_dir)
    if zip_output.exists():
        zip_output.unlink()
    if sha_output.exists():
        sha_output.unlink()

    staging_dir.mkdir(parents=True, exist_ok=True)
    print(f"\n[1/4] Merakit berkas distribusi ke: {staging_dir}")

    # 1. Root files
    root_files = [
        "MedVault.exe",
        "run_app.bat",
        "VERSION",
        "README.md",
        "CHANGELOG.md",
        "requirements.txt",
        ".env.example",
    ]
    for rf in root_files:
        src = PROJECT_ROOT / rf
        if src.is_file():
            shutil.copy2(src, staging_dir / rf)
            print(f"  + Berkas: {rf}")

    # 2. Backend files (exclude pycache)
    backend_src = PROJECT_ROOT / "backend"
    backend_dst = staging_dir / "backend"
    copy_tree_filtered(backend_src, backend_dst, {"__pycache__"})
    print("  + Direktori: backend/")

    # 3. Scripts
    scripts_src = PROJECT_ROOT / "scripts"
    scripts_dst = staging_dir / "scripts"
    copy_tree_filtered(scripts_src, scripts_dst, {"__pycache__"})
    print("  + Direktori: scripts/")

    # 4. Storage structure with gitkeeps
    for sub in ["", "biometrics", "encrypted_labs", "stego_scans"]:
        sub_dir = staging_dir / "storage" / sub
        sub_dir.mkdir(parents=True, exist_ok=True)
        gitkeep = PROJECT_ROOT / "storage" / sub / ".gitkeep"
        if gitkeep.is_file():
            shutil.copy2(gitkeep, sub_dir / ".gitkeep")
    print("  + Direktori: storage/ (struktur bersih)")

    # 5. Frontend Desktop (production bundle + electron runtime)
    frontend_staging = staging_dir / "frontend_desktop"
    frontend_staging.mkdir(parents=True, exist_ok=True)

    fe_root_files = ["package.json", ".npmrc", "pnpm-lock.yaml"]
    for fe_f in fe_root_files:
        fe_src = PROJECT_ROOT / "frontend_desktop" / fe_f
        if fe_src.is_file():
            shutil.copy2(fe_src, frontend_staging / fe_f)

    # Copy dist and electron
    fe_dist = PROJECT_ROOT / "frontend_desktop" / "dist"
    if fe_dist.is_dir():
        copy_tree_filtered(fe_dist, frontend_staging / "dist", set())
        print("  + Direktori: frontend_desktop/dist (React production build)")

    fe_electron = PROJECT_ROOT / "frontend_desktop" / "electron"
    if fe_electron.is_dir():
        copy_tree_filtered(fe_electron, frontend_staging / "electron", set())
        print("  + Direktori: frontend_desktop/electron")

    # 6. Create ZIP archive
    print(f"\n[2/4] Mengompresi paket rilis ke: {zip_output.name}")
    with zipfile.ZipFile(zip_output, "w", zipfile.ZIP_DEFLATED) as zipf:
        for file_path in staging_dir.rglob("*"):
            if file_path.is_file():
                arcname = file_path.relative_to(dist_packages)
                zipf.write(file_path, arcname)

    zip_size_mb = zip_output.stat().st_size / (1024 * 1024)
    print(f"[OK] Arsip terkompresi: {zip_size_mb:.2f} MB")

    # 7. Compute SHA-256 Checksum
    print("\n[3/4] Menghitung checksum SHA-256...")
    sha256_hash = compute_sha256(zip_output)
    sha_content = f"{sha256_hash}  {zip_output.name}\n"
    sha_output.write_text(sha_content, encoding="utf-8")
    print(f"[OK] Checksum: {sha256_hash}")

    print("\n[4/4] Paket rilis siap diunggah ke GitHub!")
    print("=" * 70)
    print(f"Versi: v{version}")
    print(f"Berkas ZIP: {zip_output}")
    print(f"Berkas SHA: {sha_output}")
    print("=" * 70)


if __name__ == "__main__":
    main()
