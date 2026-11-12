// Package ledger implements the dual signature Proof-of-Authority audit chain.
//
// A block is sealed when two independent Ed25519 signatures exist over the same
// canonical payload: one by the doctor's USB dongle key and one by the
// validator gateway. The package has no net/http dependency; the HTTP proposal
// client lives in client.go and is used only by the application process.
package ledger

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Action types recorded on the chain.
const (
	ActionGenesis      = "GENESIS"
	ActionCreateRecord = "CREATE_RECORD"
	ActionUpdateRecord = "UPDATE_RECORD"
	ActionUploadLab    = "UPLOAD_LAB"
	ActionEmbedStego   = "EMBED_STEGO"
)

// GeneisPreviousHash is the previous hash of block zero.
var GeneisPreviousHash = strings.Repeat("0", 64)

// Block mirrors one row of the audit_blocks table.
type Block struct {
	BlockIndex            int64  `json:"block_index"`
	Timestamp             string `json:"timestamp"`
	ActorUsername         string `json:"actor_username"`
	ActorPubkeyHex        string `json:"actor_pubkey_hex"`
	ActionType            string `json:"action_type"`
	TargetRecordID        *int64 `json:"target_record_id,omitempty"`
	RecordStateHash       string `json:"record_state_hash"`
	PreviousHash          string `json:"previous_hash"`
	DoctorSignatureHex    string `json:"doctor_signature_hex"`
	ValidatorPubkeyHex    string `json:"validator_pubkey_hex"`
	ValidatorSignatureHex string `json:"validator_signature_hex"`
	CurrentHash           string `json:"current_hash"`
}

// TxPayload is the exact byte string the doctor signs.
//
// The format is fixed by context/004 and must not change, otherwise every
// historic signature becomes unverifiable.
func TxPayload(b *Block) []byte {
	target := int64(0)
	if b.TargetRecordID != nil {
		target = *b.TargetRecordID
	}
	return []byte(fmt.Sprintf(
		"%d|%s|%s|%d|%s|%s",
		b.BlockIndex,
		b.Timestamp,
		b.ActorUsername,
		target,
		b.ActionType,
		b.RecordStateHash,
	))
}

// hashPayload is the canonical JSON projection that is hashed into
// CurrentHash. The validator signature is deliberately excluded so the
// validator can compute the hash before it countersigns.
type hashPayload struct {
	BlockIndex         int64  `json:"block_index"`
	Timestamp          string `json:"timestamp"`
	ActorUsername      string `json:"actor_username"`
	ActorPubkeyHex     string `json:"actor_pubkey_hex"`
	ActionType         string `json:"action_type"`
	TargetRecordID     *int64 `json:"target_record_id,omitempty"`
	RecordStateHash    string `json:"record_state_hash"`
	PreviousHash       string `json:"previous_hash"`
	DoctorSignatureHex string `json:"doctor_signature_hex"`
}

// ComputeCurrentHash returns the SHA-256 hex of the canonical block payload.
func (b *Block) ComputeCurrentHash() (string, error) {
	payload := hashPayload{
		BlockIndex:         b.BlockIndex,
		Timestamp:          b.Timestamp,
		ActorUsername:      b.ActorUsername,
		ActorPubkeyHex:     b.ActorPubkeyHex,
		ActionType:         b.ActionType,
		TargetRecordID:     b.TargetRecordID,
		RecordStateHash:    b.RecordStateHash,
		PreviousHash:       b.PreviousHash,
		DoctorSignatureHex: b.DoctorSignatureHex,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal canonical block payload: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// Signer wraps an Ed25519 private key with helpers.
type Signer struct {
	private ed25519.PrivateKey
}

// NewSigner generates a fresh key pair.
func NewSigner() (*Signer, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	return &Signer{private: priv}, nil
}

// NewSignerFromSeed reconstructs a signer from a 32 byte hex seed.
func NewSignerFromSeed(seedHex string) (*Signer, error) {
	seed, err := hex.DecodeString(strings.TrimSpace(seedHex))
	if err != nil {
		return nil, fmt.Errorf("seed is not hex: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	return &Signer{private: ed25519.NewKeyFromSeed(seed)}, nil
}

// PublicKeyHex returns the hex encoded public key.
func (s *Signer) PublicKeyHex() string {
	return hex.EncodeToString(s.private.Public().(ed25519.PublicKey))
}

// Sign returns the hex encoded signature over message.
func (s *Signer) Sign(message []byte) string {
	return hex.EncodeToString(ed25519.Sign(s.private, message))
}

// NewGenesis builds the unsigned genesis block.
func NewGenesis(actorUsername, actorPubkeyHex string) (*Block, error) {
	if actorUsername == "" {
		return nil, errors.New("genesis actor username must not be empty")
	}
	if actorPubkeyHex == "" {
		return nil, errors.New("genesis actor public key must not be empty")
	}

	genesis := &Block{
		BlockIndex:      0,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ActorUsername:   actorUsername,
		ActorPubkeyHex:  actorPubkeyHex,
		ActionType:      ActionGenesis,
		TargetRecordID:  nil,
		RecordStateHash: strings.Repeat("0", 64),
		PreviousHash:    GeneisPreviousHash,
	}

	hash, err := genesis.ComputeCurrentHash()
	if err != nil {
		return nil, err
	}
	genesis.CurrentHash = hash

	return genesis, nil
}

// SignAsDoctor signs the transaction payload with the doctor key.
func SignAsDoctor(b *Block, signer *Signer) error {
	if signer == nil {
		return errors.New("doctor signer is nil")
	}
	if b.ActorPubkeyHex != signer.PublicKeyHex() {
		return errors.New("block actor public key does not match the signing key")
	}

	b.DoctorSignatureHex = signer.Sign(TxPayload(b))

	hash, err := b.ComputeCurrentHash()
	if err != nil {
		return err
	}
	b.CurrentHash = hash
	return nil
}

// SignAsValidator countersigns the current hash with the validator key.
func SignAsValidator(b *Block, signer *Signer) error {
	if signer == nil {
		return errors.New("validator signer is nil")
	}

	// Recompute so a stale hash can never be countersigned.
	hash, err := b.ComputeCurrentHash()
	if err != nil {
		return err
	}
	b.CurrentHash = hash

	b.ValidatorPubkeyHex = signer.PublicKeyHex()
	b.ValidatorSignatureHex = signer.Sign([]byte(hash))
	return nil
}

// VerifyBlockSignature checks the doctor signature over the transaction payload.
func VerifyBlockSignature(b *Block) error {
	if b.DoctorSignatureHex == "" {
		return errors.New("block has no doctor signature")
	}

	pub, err := hex.DecodeString(b.ActorPubkeyHex)
	if err != nil {
		return fmt.Errorf("actor public key is not hex: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("actor public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}

	sig, err := hex.DecodeString(b.DoctorSignatureHex)
	if err != nil {
		return fmt.Errorf("doctor signature is not hex: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("doctor signature must be %d bytes, got %d", ed25519.SignatureSize, len(sig))
	}

	if !ed25519.Verify(ed25519.PublicKey(pub), TxPayload(b), sig) {
		return fmt.Errorf("doctor signature verification failed at block %d", b.BlockIndex)
	}
	return nil
}

// VerifyValidatorSignature checks the countersignature over CurrentHash.
func VerifyValidatorSignature(b *Block) error {
	if b.ValidatorSignatureHex == "" {
		return fmt.Errorf("block %d has no validator countersignature", b.BlockIndex)
	}

	pub, err := hex.DecodeString(b.ValidatorPubkeyHex)
	if err != nil {
		return fmt.Errorf("validator public key is not hex: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("validator public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}

	sig, err := hex.DecodeString(b.ValidatorSignatureHex)
	if err != nil {
		return fmt.Errorf("validator signature is not hex: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("validator signature must be %d bytes, got %d", ed25519.SignatureSize, len(sig))
	}

	if !ed25519.Verify(ed25519.PublicKey(pub), []byte(b.CurrentHash), sig) {
		return fmt.Errorf("validator countersignature verification failed at block %d", b.BlockIndex)
	}
	return nil
}
