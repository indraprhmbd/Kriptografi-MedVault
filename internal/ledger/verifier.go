package ledger

import (
	"errors"
	"fmt"
	"strings"
)

// RecordHashFunc resolves the live cryptographic fingerprint of a record so the
// verifier can detect a database edited outside the application.
//
// It returns the current SHA-256 hex of the record state. An empty string means
// the record no longer exists.
type RecordHashFunc func(blockIndex, recordID int64) (string, error)

// ChainFailure identifies exactly where verification stopped.
type ChainFailure struct {
	BlockIndex int64  `json:"block_index"`
	Reason     string `json:"reason"`
	Detail     string `json:"detail,omitempty"`
}

// ChainResult is the verification outcome returned by VerifyChain.
type ChainResult struct {
	Valid         bool          `json:"valid"`
	BlocksChecked int           `json:"blocks_checked"`
	FirstFailure  *ChainFailure `json:"first_failure,omitempty"`
	Steps         []string      `json:"steps"`
}

// VerifyChain walks the chain in index order and validates:
//  1. the genesis previous hash is all zeros,
//  2. the index sequence has no gaps and starts at zero,
//  3. every previous hash equals the prior block's current hash,
//  4. the recomputed current hash matches the stored one,
//  5. the doctor signature is valid over the transaction payload,
//  6. the validator countersignature is valid over the current hash,
//  7. when a record callback is supplied, the live record fingerprint still
//     matches what the block sealed.
func VerifyChain(blocks []Block, live RecordHashFunc) (*ChainResult, error) {
	if len(blocks) == 0 {
		return nil, errors.New("chain is empty")
	}

	result := &ChainResult{Valid: true, Steps: make([]string, 0, len(blocks)*6)}

	fail := func(index int64, reason, detail string) *ChainResult {
		result.Valid = false
		result.FirstFailure = &ChainFailure{BlockIndex: index, Reason: reason, Detail: detail}
		return result
	}

	for position, block := range blocks {
		result.BlocksChecked = position + 1

		if position == 0 {
			if block.PreviousHash != GeneisPreviousHash {
				return fail(block.BlockIndex, "GENESIS_PREVIOUS_HASH", "genesis previous hash must be 64 zeros"), nil
			}
			if block.BlockIndex != 0 {
				return fail(block.BlockIndex, "GENESIS_INDEX", "first block index must be 0"), nil
			}
			result.Steps = append(result.Steps, "genesis prefix verified")
		} else {
			prev := blocks[position-1]

			if block.BlockIndex != prev.BlockIndex+1 {
				return fail(block.BlockIndex, "INDEX_GAP",
					fmt.Sprintf("expected index %d, got %d", prev.BlockIndex+1, block.BlockIndex)), nil
			}

			if !strings.EqualFold(block.PreviousHash, prev.CurrentHash) {
				return fail(block.BlockIndex, "BROKEN_LINK",
					fmt.Sprintf("previous hash %s does not match block %d hash %s",
						short(block.PreviousHash), prev.BlockIndex, short(prev.CurrentHash))), nil
			}
			result.Steps = append(result.Steps, fmt.Sprintf("link %d to %d verified", prev.BlockIndex, block.BlockIndex))
		}

		recomputed, err := block.ComputeCurrentHash()
		if err != nil {
			return fail(block.BlockIndex, "HASH_ERROR", err.Error()), nil
		}
		if !strings.EqualFold(recomputed, block.CurrentHash) {
			return fail(block.BlockIndex, "HASH_MISMATCH",
				fmt.Sprintf("stored %s, recomputed %s", short(block.CurrentHash), short(recomputed))), nil
		}
		result.Steps = append(result.Steps, fmt.Sprintf("block %d hash recomputed", block.BlockIndex))

		if err := VerifyBlockSignature(&block); err != nil {
			return fail(block.BlockIndex, "DOCTOR_SIGNATURE", err.Error()), nil
		}
		result.Steps = append(result.Steps, fmt.Sprintf("block %d doctor signature verified", block.BlockIndex))

		if err := VerifyValidatorSignature(&block); err != nil {
			return fail(block.BlockIndex, "VALIDATOR_SIGNATURE", err.Error()), nil
		}
		result.Steps = append(result.Steps, fmt.Sprintf("block %d validator countersignature verified", block.BlockIndex))

		if live != nil && block.TargetRecordID != nil {
			currentHash, err := live(block.BlockIndex, *block.TargetRecordID)
			if err != nil {
				return fail(block.BlockIndex, "RECORD_LOOKUP", err.Error()), nil
			}
			if currentHash == "" {
				return fail(block.BlockIndex, "RECORD_MISSING",
					fmt.Sprintf("record %d no longer exists", *block.TargetRecordID)), nil
			}
			if !strings.EqualFold(currentHash, block.RecordStateHash) {
				return fail(block.BlockIndex, "RECORD_TAMPERED",
					fmt.Sprintf("record %d live hash %s does not match sealed %s",
						*block.TargetRecordID, short(currentHash), short(block.RecordStateHash))), nil
			}
			result.Steps = append(result.Steps,
				fmt.Sprintf("block %d sealed record %d still matches", block.BlockIndex, *block.TargetRecordID))
		}
	}

	return result, nil
}

// short truncates a hash for human readable failure details.
func short(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12] + "..."
}
