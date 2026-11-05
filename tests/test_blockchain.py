"""Unit tests for Blockchain Audit Ledger (backend/core/blockchain.py).

Follows Test-Driven Development (TDD) principles to verify:
1. Deterministic block hash computation using canonical JSON
2. Genesis block validation (previous_hash = 64 zeros)
3. Sequential block addition and chain linking
4. Tamper detection at exact block index when record payload or hash is manipulated
"""

from datetime import datetime, timezone
import pytest

from backend.core.blockchain import Block, BlockchainLedger, compute_block_hash


@pytest.fixture
def clean_ledger() -> BlockchainLedger:
    ledger = BlockchainLedger()
    ledger.create_genesis_block(timestamp="2026-11-04T00:00:00Z")
    return ledger


class TestBlockchainLedger:
    """Tests for immutable SHA-256 blockchain audit trail."""

    def test_compute_block_hash_deterministic(self):
        h1 = compute_block_hash(
            block_index=1,
            timestamp="2026-11-04T10:00:00Z",
            actor_username="dr_budi",
            action_type="CREATE_RECORD",
            target_record_id=101,
            change_summary="New diagnosis recorded",
            previous_hash="a" * 64,
        )
        h2 = compute_block_hash(
            block_index=1,
            timestamp="2026-11-04T10:00:00Z",
            actor_username="dr_budi",
            action_type="CREATE_RECORD",
            target_record_id=101,
            change_summary="New diagnosis recorded",
            previous_hash="a" * 64,
        )
        assert len(h1) == 64
        assert h1 == h2

    def test_genesis_block_structure(self, clean_ledger: BlockchainLedger):
        genesis = clean_ledger.get_block(0)
        assert genesis is not None
        assert genesis.block_index == 0
        assert genesis.action_type == "GENESIS"
        assert genesis.previous_hash == "0" * 64
        assert len(genesis.current_hash) == 64

    def test_valid_chain_verification(self, clean_ledger: BlockchainLedger):
        clean_ledger.add_block(
            actor_username="dr_budi",
            action_type="CREATE_RECORD",
            target_record_id=1,
            change_summary="Patient RM-001 created with AES-128 PII",
            timestamp="2026-11-04T10:05:00Z",
        )
        clean_ledger.add_block(
            actor_username="dr_budi",
            action_type="UPDATE_RECORD",
            target_record_id=1,
            change_summary="Super-encrypted diagnosis updated",
            timestamp="2026-11-04T10:10:00Z",
        )

        assert len(clean_ledger.blocks) == 3
        report = clean_ledger.verify_chain()
        assert report["is_valid"] is True
        assert report["total_blocks"] == 3
        assert report["broken_at_index"] is None

    def test_tamper_detection_in_block_summary(self, clean_ledger: BlockchainLedger):
        clean_ledger.add_block(
            actor_username="dr_budi",
            action_type="CREATE_RECORD",
            target_record_id=1,
            change_summary="Original diagnosis: Normal",
            timestamp="2026-11-04T10:05:00Z",
        )
        clean_ledger.add_block(
            actor_username="dr_budi",
            action_type="UPLOAD_LAB",
            target_record_id=1,
            change_summary="Lab PDF encrypted with ChaCha20",
            timestamp="2026-11-04T10:10:00Z",
        )

        # Simulate tampering on block #1
        clean_ledger.blocks[1].change_summary = "Tampered diagnosis: Malicious alter"

        report = clean_ledger.verify_chain()
        assert report["is_valid"] is False
        assert report["broken_at_index"] == 1
        detail = report["error_detail"].lower()
        assert "manipulasi" in detail or "tamper" in detail or "tidak sesuai" in detail
