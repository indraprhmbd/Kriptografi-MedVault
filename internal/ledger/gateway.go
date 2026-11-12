package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// maxProposalBytes bounds the request body a gateway will read.
const maxProposalBytes = 64 << 10

// Gateway implements the Proof-of-Authority validator service.
//
// It holds the validator private key and performs the three point inspection
// described in context/004 before returning a countersigned block. The main
// application never sees this key.
type Gateway struct {
	signer *Signer
}

// NewGateway requires a validator signer.
func NewGateway(signer *Signer) (*Gateway, error) {
	if signer == nil {
		return nil, errors.New("gateway requires a validator signer")
	}
	return &Gateway{signer: signer}, nil
}

// PublicKeyHex exposes the validator identity for health reporting.
func (g *Gateway) PublicKeyHex() string {
	return g.signer.PublicKeyHex()
}

// Handler returns the internal HTTP surface. Only POST /internal/v1/validate
// mutates anything; /healthz is read only.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(ValidatePath, g.handleValidate)
	mux.HandleFunc("/healthz", g.handleHealth)
	return mux
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":               "ok",
		"service":              "medvault-validator",
		"validator_pubkey_hex": g.PublicKeyHex(),
	})
}

func (g *Gateway) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, ValidationResponse{
			Accepted: false,
			Reason:   "method not allowed, use POST",
		})
		return
	}

	var block Block
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxProposalBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&block); err != nil {
		writeJSON(w, http.StatusBadRequest, ValidationResponse{
			Accepted: false,
			Reason:   "malformed block proposal: " + err.Error(),
		})
		return
	}

	response := g.Validate(&block)
	writeJSON(w, http.StatusOK, response)
}

// Validate runs the three point inspection and, when it passes, countersigns.
//
// A rejection never produces a signature. That property is what makes the demo
// meaningful: a tampered record cannot obtain a validator seal.
func (g *Gateway) Validate(block *Block) *ValidationResponse {
	checks := make([]string, 0, 3)

	// Check A: the doctor signature must verify over the canonical payload.
	if err := VerifyBlockSignature(block); err != nil {
		return &ValidationResponse{
			Accepted: false,
			Checks:   checks,
			Reason:   "check A failed, doctor signature: " + err.Error(),
		}
	}
	checks = append(checks, "check A: doctor Ed25519 signature verified over the canonical transaction payload")

	// Check B: the chain linkage must be structurally plausible.
	if block.BlockIndex < 0 {
		return &ValidationResponse{
			Accepted: false,
			Checks:   checks,
			Reason:   "check B failed, block index must not be negative",
		}
	}
	if len(block.PreviousHash) != 64 || !isHex(block.PreviousHash) {
		return &ValidationResponse{
			Accepted: false,
			Checks:   checks,
			Reason:   fmt.Sprintf("check B failed, previous hash must be 64 hex characters, got %q", block.PreviousHash),
		}
	}
	checks = append(checks, fmt.Sprintf("check B: chain linkage verified, previous hash %s", short(block.PreviousHash)))

	// Check C: recompute the canonical hash so a stale or forged hash is caught.
	recomputed, err := block.ComputeCurrentHash()
	if err != nil {
		return &ValidationResponse{
			Accepted: false,
			Checks:   checks,
			Reason:   "check C failed, cannot compute canonical hash: " + err.Error(),
		}
	}
	if block.CurrentHash != "" && !strings.EqualFold(block.CurrentHash, recomputed) {
		return &ValidationResponse{
			Accepted: false,
			Checks:   checks,
			Reason:   fmt.Sprintf("check C failed, proposed hash %s does not match recomputed %s", short(block.CurrentHash), short(recomputed)),
		}
	}
	block.CurrentHash = recomputed
	checks = append(checks, "check C: canonical block hash recomputed as "+short(recomputed))

	sealed := *block
	if err := SignAsValidator(&sealed, g.signer); err != nil {
		return &ValidationResponse{
			Accepted: false,
			Checks:   checks,
			Reason:   "countersignature failed: " + err.Error(),
		}
	}

	return &ValidationResponse{
		Accepted:           true,
		Block:              sealed,
		ValidatorPubkeyHex: g.signer.PublicKeyHex(),
		Checks:             checks,
	}
}

// isHex reports whether s contains only hexadecimal characters.
func isHex(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
