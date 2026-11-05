@echo off
title MedVault Crypto - Standalone Desktop EMR
cd /d "%~dp0"

echo ======================================================================
echo  Memulai MedVault Crypto - Standalone Desktop EMR
echo ======================================================================

if exist ".\.venv\Scripts\python.exe" (
    ".\.venv\Scripts\python.exe" scripts\run_app.py
) else (
    python scripts\run_app.py
)

pause
