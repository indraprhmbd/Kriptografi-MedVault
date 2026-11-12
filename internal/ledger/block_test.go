package ledger

import (
	"strings"
	"testing"
)

func ptr(v int64) *int64 { return &v }

// newTestBlock builds an unsigned block for chain tests.
func newTestBlock(index int64, prevHash, stateHash, action string) *Block {
	return &Block{
		BlockIndex:      index,
		Timestamp:       "2026-11-12T10:00:00Z",
		ActorUsername:   "dr_andi",
		ActorPubkeyHex:  strings.Repeat("a1", 32),
		ActionType:      action,
		TargetRecordID:  ptr(1),
		RecordStateHash: stateHash,
		PreviousHash:    prevHash,
	}
}

func TestGenesisBlockIsDeterministic(t *testing.T) {
	first, err := NewGenesis("dr_andi", strings.Repeat("a1", 32))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	second, err := NewGenesis("dr_andi", strings.Repeat("a1", 32))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}

	if first.BlockIndex != 0 {
		t.Fatalf("genesis index must be 0, got %d", first.BlockIndex)
	}
	if first.PreviousHash != strings.Repeat("0", 64) {
		t.Fatalf("genesis previous hash must be 64 zeros, got %q", first.PreviousHash)
	}
	if first.CurrentHash == "" || first.CurrentHash != second.CurrentHash {
		t.Fatal("genesis hash must be deterministic for identical input")
	}
}

func TestComputeCurrentHashIsSensitiveToEveryField(t *testing.T) {
	base := newTestBlock(1, strings.Repeat("0", 64), strings.Repeat("c3", 32), ActionCreateRecord)

	baseline, err := base.ComputeCurrentHash()
	if err != nil {
		t.Fatalf("ComputeCurrentHash: %v", err)
	}

	mutations := map[string]func(*Block){
		"index":      func(b *Block) { b.BlockIndex = 2 },
		"timestamp":  func(b *Block) { b.Timestamp = "2026-11-12T11:00:00Z" },
		"actor":      func(b *Block) { b.ActorUsername = "dr_budi" },
		"pubkey":     func(b *Block) { b.ActorPubkeyHex = strings.Repeat("b2", 32) },
		"action":     func(b *Block) { b.ActionType = ActionUpdateRecord },
		"target":     func(b *Block) { b.TargetRecordID = ptr(99) },
		"state":      func(b *Block) { b.RecordStateHash = strings.Repeat("d4", 32) },
		"prev":       func(b *Block) { b.PreviousHash = strings.Repeat("e5", 32) },
		"doctor_sig": func(b *Block) { b.DoctorSignatureHex = strings.Repeat("f6", 64) },
	}

	for name, mutate := range mutations {
		clone := *base
		mutate(&clone)
		got, err := clone.ComputeCurrentHash()
		if err != nil {
			t.Fatalf("%s: ComputeCurrentHash: %v", name, err)
		}
		if got == baseline {
			t.Fatalf("mutating %s must change the current hash", name)
		}
	}
}

func TestCurrentHashIgnoresValidatorSignature(t *testing.T) {
	block := newTestBlock(1, strings.Repeat("0", 64), strings.Repeat("c3", 32), ActionCreateRecord)

	before, err := block.ComputeCurrentHash()
	if err != nil {
		t.Fatalf("ComputeCurrentHash: %v", err)
	}

	block.ValidatorSignatureHex = strings.Repeat("9a", 64)
	block.ValidatorPubkeyHex = strings.Repeat("8b", 32)

	after, err := block.ComputeCurrentHash()
	if err != nil {
		t.Fatalf("ComputeCurrentHash: %v", err)
	}

	if before != after {
		t.Fatal("the validator countersignature must not be part of the hashed payload")
	}
}

func TestTxPayloadFormat(t *testing.T) {
	block := newTestBlock(7, strings.Repeat("0", 64), "statehash", ActionCreateRecord)

	got := string(TxPayload(block))
	want := "7|2026-11-12T10:00:00Z|dr_andi|1|CREATE_RECORD|statehash"

	if got != want {
		t.Fatalf("tx payload mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestTxPayloadHandlesNilTarget(t *testing.T) {
	block := newTestBlock(1, strings.Repeat("0", 64), "state", ActionGenesis)
	block.TargetRecordID = nil

	got := string(TxPayload(block))
	if !strings.Contains(got, "|0|") {
		t.Fatalf("nil target must render as 0, got %q", got)
	}
}

func TestDualSignatureFlow(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	block := newTestBlock(0, strings.Repeat("0", 64), strings.Repeat("c3", 32), ActionGenesis)
	block.ActorPubkeyHex = doctor.PublicKeyHex()

	if err := SignAsDoctor(block, doctor); err != nil {
		t.Fatalf("SignAsDoctor: %v", err)
	}
	if block.DoctorSignatureHex == "" {
		t.Fatal("doctor signature must be set")
	}

	if err := SignAsValidator(block, validator); err != nil {
		t.Fatalf("SignAsValidator: %v", err)
	}
	if block.ValidatorSignatureHex == "" {
		t.Fatal("validator signature must be set")
	}
	if block.ValidatorPubkeyHex != validator.PublicKeyHex() {
		t.Fatal("validator public key must be recorded on the block")
	}

	if err := VerifyBlockSignature(block); err != nil {
		t.Fatalf("VerifyBlockSignature: %v", err)
	}
}

func TestVerifyBlockSignatureDetectsTampering(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}

	block := newTestBlock(1, strings.Repeat("0", 64), strings.Repeat("c3", 32), ActionCreateRecord)
	block.ActorPubkeyHex = doctor.PublicKeyHex()

	if err := SignAsDoctor(block, doctor); err != nil {
		t.Fatalf("SignAsDoctor: %v", err)
	}
	if err := VerifyBlockSignature(block); err != nil {
		t.Fatalf("baseline verify: %v", err)
	}

	block.RecordStateHash = strings.Repeat("ff", 32)

	if err := VerifyBlockSignature(block); err == nil {
		t.Fatal("verify must fail after the record state hash is altered")
	}
}

func TestVerifyChainAcceptsValidChain(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	chain := BuildTestChain(t, doctor, validator, 3)

	result, err := VerifyChain(chain, nil)
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if !result.Valid {
		t.Fatalf("chain must be valid, first failure: %+v", result.FirstFailure)
	}
	if result.BlocksChecked != 3 {
		t.Fatalf("expected 3 blocks checked, got %d", result.BlocksChecked)
	}
}

func TestVerifyChainDetectsBrokenLink(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	chain := BuildTestChain(t, doctor, validator, 3)
	chain[2].PreviousHash = strings.Repeat("00", 32)

	result, err := VerifyChain(chain, nil)
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if result.Valid {
		t.Fatal("chain with a broken previous hash must be invalid")
	}
	if result.FirstFailure.BlockIndex != chain[2].BlockIndex {
		t.Fatalf("failure must point at the broken block, got %d", result.FirstFailure.BlockIndex)
	}
}

func TestVerifyChainDetectsIndexGap(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	chain := BuildTestChain(t, doctor, validator, 3)
	chain[1].BlockIndex = 5

	result, err := VerifyChain(chain, nil)
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if result.Valid {
		t.Fatal("chain with an index gap must be invalid")
	}
}

func TestVerifyChainDetectsRecordTamperThroughCallback(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	chain := BuildTestChain(t, doctor, validator, 2)

	// Simulate an attacker editing the records table directly: the live hash no
	// longer matches what the block sealed.
	live := func(blockIndex, recordID int64) (string, error) {
		return strings.Repeat("77", 32), nil
	}

	result, err := VerifyChain(chain, live)
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if result.Valid {
		t.Fatal("chain must be invalid when the live record hash differs")
	}
}

func TestVerifyChainRejectsEmptyChain(t *testing.T) {
	if _, err := VerifyChain(nil, nil); err == nil {
		t.Fatal("expected error for an empty chain")
	}
}

func TestVerifyChainRejectsWrongGenesisPreviousHash(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	chain := BuildTestChain(t, doctor, validator, 2)
	chain[0].PreviousHash = strings.Repeat("ab", 32)

	result, err := VerifyChain(chain, nil)
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if result.Valid {
		t.Fatal("genesis previous hash must be all zeros")
	}
}

func TestVerifyChainDetectsAlteredCurrentHash(t *testing.T) {
	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	chain := BuildTestChain(t, doctor, validator, 2)
	chain[0].CurrentHash = strings.Repeat("99", 32)

	result, err := VerifyChain(chain, nil)
	if err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	if result.Valid {
		t.Fatal("a recomputed hash mismatch must invalidate the chain")
	}
}

// BuildTestChain constructs a signed chain for tests.
func BuildTestChain(t *testing.T, doctor, validator *Signer, length int) []Block {
	t.Helper()

	genesis, err := NewGenesis("dr_andi", doctor.PublicKeyHex())
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	if err := SignAsDoctor(genesis, doctor); err != nil {
		t.Fatalf("genesis doctor signature: %v", err)
	}
	if err := SignAsValidator(genesis, validator); err != nil {
		t.Fatalf("genesis validator signature: %v", err)
	}

	chain := []Block{*genesis}

	for i := 1; i < length; i++ {
		prev := chain[len(chain)-1]
		block := newTestBlock(int64(i), prev.CurrentHash, strings.Repeat("c3", 32), ActionCreateRecord)
		block.ActorPubkeyHex = doctor.PublicKeyHex()

		if err := SignAsDoctor(block, doctor); err != nil {
			t.Fatalf("block %d doctor signature: %v", i, err)
		}
		if err := SignAsValidator(block, validator); err != nil {
			t.Fatalf("block %d validator signature: %v", i, err)
		}
		chain = append(chain, *block)
	}

	return chain
}
