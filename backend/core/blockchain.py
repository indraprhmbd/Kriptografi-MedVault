"""Blockchain Audit Ledger for MedVault.

Provides an immutable sequential hash chain logging all clinical record actions
with tamper verification to prove database integrity.
"""

from dataclasses import asdict, dataclass
from datetime import datetime, timezone
import hashlib
import json
from typing import Any, Optional


def compute_block_hash(
    block_index: int,
    timestamp: str,
    actor_username: str,
    action_type: str,
    target_record_id: Optional[int],
    change_summary: str,
    previous_hash: str,
) -> str:
    """Compute deterministic SHA-256 digest over canonical JSON representation."""
    canonical = json.dumps(
        {
            "action_type": action_type,
            "actor_username": actor_username,
            "block_index": block_index,
            "change_summary": change_summary,
            "previous_hash": previous_hash,
            "target_record_id": target_record_id,
            "timestamp": timestamp,
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


@dataclass
class Block:
    """Represents a single immutable entry in the audit ledger."""

    block_index: int
    timestamp: str
    actor_username: str
    action_type: str
    target_record_id: Optional[int]
    change_summary: str
    previous_hash: str
    current_hash: str

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


class BlockchainLedger:
    """In-memory blockchain ledger manager and verification engine."""

    def __init__(self, blocks: Optional[list[Block]] = None) -> None:
        self.blocks: list[Block] = blocks or []

    def create_genesis_block(
        self,
        timestamp: Optional[str] = None,
        summary: str = "MedVault Genesis Block - System Initialized",
    ) -> Block:
        """Create and append the Genesis block (index 0)."""
        ts = timestamp or datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        prev_hash = "0" * 64
        actor = "SYSTEM"
        action = "GENESIS"
        target_id = None

        current_hash = compute_block_hash(
            block_index=0,
            timestamp=ts,
            actor_username=actor,
            action_type=action,
            target_record_id=target_id,
            change_summary=summary,
            previous_hash=prev_hash,
        )

        genesis = Block(
            block_index=0,
            timestamp=ts,
            actor_username=actor,
            action_type=action,
            target_record_id=target_id,
            change_summary=summary,
            previous_hash=prev_hash,
            current_hash=current_hash,
        )
        self.blocks = [genesis]
        return genesis

    def add_block(
        self,
        actor_username: str,
        action_type: str,
        target_record_id: Optional[int],
        change_summary: str,
        timestamp: Optional[str] = None,
    ) -> Block:
        """Append a new block chained to the latest block."""
        if not self.blocks:
            self.create_genesis_block()

        last_block = self.blocks[-1]
        new_index = last_block.block_index + 1
        prev_hash = last_block.current_hash
        ts = timestamp or datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        curr_hash = compute_block_hash(
            block_index=new_index,
            timestamp=ts,
            actor_username=actor_username,
            action_type=action_type,
            target_record_id=target_record_id,
            change_summary=change_summary,
            previous_hash=prev_hash,
        )

        new_block = Block(
            block_index=new_index,
            timestamp=ts,
            actor_username=actor_username,
            action_type=action_type,
            target_record_id=target_record_id,
            change_summary=change_summary,
            previous_hash=prev_hash,
            current_hash=curr_hash,
        )
        self.blocks.append(new_block)
        return new_block

    def get_block(self, index: int) -> Optional[Block]:
        """Retrieve block by index."""
        if 0 <= index < len(self.blocks):
            return self.blocks[index]
        return None

    def verify_chain(self) -> dict[str, Any]:
        """Validate sequential hash chaining and detect database tampering."""
        if not self.blocks:
            return {
                "is_valid": False,
                "total_blocks": 0,
                "broken_at_index": None,
                "error_detail": "Rantai blok kosong (tidak ada Genesis Block)",
            }

        # 1. Verify Genesis block
        genesis = self.blocks[0]
        if genesis.block_index != 0:
            return {
                "is_valid": False,
                "total_blocks": len(self.blocks),
                "broken_at_index": 0,
                "error_detail": "Index Genesis Block tidak sama dengan 0",
            }
        if genesis.previous_hash != "0" * 64:
            return {
                "is_valid": False,
                "total_blocks": len(self.blocks),
                "broken_at_index": 0,
                "error_detail": "Previous hash Genesis Block tidak valid",
            }

        expected_genesis_hash = compute_block_hash(
            block_index=genesis.block_index,
            timestamp=genesis.timestamp,
            actor_username=genesis.actor_username,
            action_type=genesis.action_type,
            target_record_id=genesis.target_record_id,
            change_summary=genesis.change_summary,
            previous_hash=genesis.previous_hash,
        )
        if genesis.current_hash != expected_genesis_hash:
            return {
                "is_valid": False,
                "total_blocks": len(self.blocks),
                "broken_at_index": 0,
                "error_detail": "Hash Genesis Block telah dimanipulasi (tamper detected)",
            }

        # 2. Verify subsequent blocks
        for i in range(1, len(self.blocks)):
            current = self.blocks[i]
            previous = self.blocks[i - 1]

            # Index continuity
            if current.block_index != previous.block_index + 1:
                return {
                    "is_valid": False,
                    "total_blocks": len(self.blocks),
                    "broken_at_index": current.block_index,
                    "error_detail": f"Urutan indeks blok terputus antara #{previous.block_index} dan #{current.block_index}",
                }

            # Chaining check: current.previous_hash == previous.current_hash
            if current.previous_hash != previous.current_hash:
                return {
                    "is_valid": False,
                    "total_blocks": len(self.blocks),
                    "broken_at_index": current.block_index,
                    "error_detail": (
                        f"Integritas rantai terputus pada Blok #{current.block_index}: "
                        f"previous_hash tidak cocok dengan current_hash Blok #{previous.block_index}"
                    ),
                }

            # Integrity check: recalculate current_hash
            expected_hash = compute_block_hash(
                block_index=current.block_index,
                timestamp=current.timestamp,
                actor_username=current.actor_username,
                action_type=current.action_type,
                target_record_id=current.target_record_id,
                change_summary=current.change_summary,
                previous_hash=current.previous_hash,
            )
            if current.current_hash != expected_hash:
                return {
                    "is_valid": False,
                    "total_blocks": len(self.blocks),
                    "broken_at_index": current.block_index,
                    "error_detail": (
                        f"Manipulasi data terdeteksi pada Blok #{current.block_index}: "
                        f"current_hash tersimpan tidak sesuai dengan kalkulasi ulang data blok"
                    ),
                }

        return {
            "is_valid": True,
            "total_blocks": len(self.blocks),
            "broken_at_index": None,
            "error_detail": None,
        }
