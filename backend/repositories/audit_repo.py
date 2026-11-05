"""Audit Repository for MedVault Blockchain Ledger.

Persists blocks into SQLite audit_blocks table and executes live chain integrity
verification to detect tampering.
"""

from datetime import datetime, timezone
from pathlib import Path
import sqlite3
from typing import Any, Optional

from backend.config import settings
from backend.core.blockchain import Block, BlockchainLedger, compute_block_hash
from backend.database import get_db_connection


class AuditRepository:
    """Data access layer for audit_blocks blockchain ledger."""

    def __init__(self, db_path: Optional[Path] = None) -> None:
        self._explicit_db_path = db_path

    @property
    def db_path(self) -> Path:
        return self._explicit_db_path or settings.db_file_path

    def append_block(
        self,
        actor_username: str,
        action_type: str,
        target_record_id: Optional[int],
        change_summary: str,
        timestamp: Optional[str] = None,
    ) -> dict[str, Any]:
        """Append a cryptographically chained block to audit_blocks table."""
        now_iso = timestamp or datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            # Fetch latest block for sequential index and previous_hash
            cursor.execute(
                "SELECT block_index, current_hash FROM audit_blocks ORDER BY block_index DESC LIMIT 1"
            )
            latest = cursor.fetchone()

            if latest:
                new_index = latest["block_index"] + 1
                previous_hash = latest["current_hash"]
            else:
                new_index = 0
                previous_hash = "0" * 64

            current_hash = compute_block_hash(
                block_index=new_index,
                timestamp=now_iso,
                actor_username=actor_username.strip(),
                action_type=action_type.strip(),
                target_record_id=target_record_id,
                change_summary=change_summary.strip(),
                previous_hash=previous_hash,
            )

            cursor.execute(
                """
                INSERT INTO audit_blocks (
                    block_index,
                    timestamp,
                    actor_username,
                    action_type,
                    target_record_id,
                    change_summary,
                    previous_hash,
                    current_hash
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    new_index,
                    now_iso,
                    actor_username.strip(),
                    action_type.strip(),
                    target_record_id,
                    change_summary.strip(),
                    previous_hash,
                    current_hash,
                ),
            )
            conn.commit()

            return {
                "block_index": new_index,
                "timestamp": now_iso,
                "actor_username": actor_username.strip(),
                "action_type": action_type.strip(),
                "target_record_id": target_record_id,
                "change_summary": change_summary.strip(),
                "previous_hash": previous_hash,
                "current_hash": current_hash,
            }
        finally:
            conn.close()

    def list_blocks(self) -> list[dict[str, Any]]:
        """Retrieve all blocks ordered by block_index ascending."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM audit_blocks ORDER BY block_index ASC")
            return [dict(r) for r in cursor.fetchall()]
        finally:
            conn.close()

    def verify_chain_integrity(self) -> dict[str, Any]:
        """Load all blocks from SQLite and verify hash chaining and tamper status."""
        raw_blocks = self.list_blocks()
        blocks = [
            Block(
                block_index=b["block_index"],
                timestamp=b["timestamp"],
                actor_username=b["actor_username"],
                action_type=b["action_type"],
                target_record_id=b["target_record_id"],
                change_summary=b["change_summary"],
                previous_hash=b["previous_hash"],
                current_hash=b["current_hash"],
            )
            for b in raw_blocks
        ]
        ledger = BlockchainLedger(blocks=blocks)
        return ledger.verify_chain()
