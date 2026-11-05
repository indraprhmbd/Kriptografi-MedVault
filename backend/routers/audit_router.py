"""Audit Ledger API Router for MedVault.

Provides endpoints to explore the sequential blockchain hash chain
and execute tamper verification checks.
"""

from fastapi import APIRouter

from backend.repositories.audit_repo import AuditRepository

router = APIRouter(prefix="/api/v1/audit", tags=["Audit Ledger"])
audit_repo = AuditRepository()


@router.get("/blocks")
def list_blocks_with_verification() -> dict:
    """Retrieve all blockchain blocks along with real-time tamper integrity verification."""
    blocks = audit_repo.list_blocks()
    integrity = audit_repo.verify_chain_integrity()

    return {
        "status": "success",
        "total_blocks": len(blocks),
        "integrity_report": integrity,
        "blocks": blocks,
    }


@router.get("/verify")
def verify_ledger() -> dict:
    """Perform cryptographically rigorous verification of every block's SHA-256 hash and previous_hash."""
    report = audit_repo.verify_chain_integrity()
    return {
        "status": "success",
        "verification": report,
        "status_label": "VALID" if report["is_valid"] else "TAMPER DETECTED",
    }
