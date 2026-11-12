package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"medvault/internal/telemetry"
)

// ValidatePath is the internal endpoint exposed by medvault-validator. It is
// only reachable from inside the Podman pod, never from the public tunnel.
const ValidatePath = "/internal/v1/validate"

// DefaultClientTimeout bounds a proposal round trip. The gateway is a local
// process, so anything slower than this means it is wedged.
const DefaultClientTimeout = 3 * time.Second

// ValidationResponse is the envelope the gateway returns.
type ValidationResponse struct {
	Accepted           bool     `json:"accepted"`
	Block              Block    `json:"block"`
	ValidatorPubkeyHex string   `json:"validator_pubkey_hex"`
	Checks             []string `json:"checks"`
	Reason             string   `json:"reason,omitempty"`
}

// Decision is the client side outcome of a proposal.
type Decision struct {
	Accepted    bool     `json:"accepted"`
	Sealed      *Block   `json:"sealed,omitempty"`
	Checks      []string `json:"checks"`
	Reason      string   `json:"reason,omitempty"`
	RoundTripMS float64  `json:"round_trip_ms"`
}

// Client proposes blocks to the validator gateway over loopback HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient validates the base URL and applies a default timeout.
func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("validator base URL must not be empty")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse validator base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("validator base URL must use http or https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, errors.New("validator base URL has no host")
	}

	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        4,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}, nil
}

// Timeout reports the effective HTTP timeout.
func (c *Client) Timeout() time.Duration {
	return c.httpClient.Timeout
}

// Propose asks the gateway to inspect and countersign a doctor signed block.
//
// A rejection is reported both as a Decision with Accepted false and as an
// error, so a caller that only checks the error still refuses to persist.
func (c *Client) Propose(ctx context.Context, block *Block) (*Decision, *telemetry.Trace, error) {
	tr := telemetry.NewTrace("POA_VALIDATOR_PROPOSAL")

	if block == nil {
		return nil, tr, errors.New("cannot propose a nil block")
	}
	if block.DoctorSignatureHex == "" {
		return nil, tr, errors.New("cannot propose a block without a doctor signature")
	}

	body, err := json.Marshal(block)
	if err != nil {
		return nil, tr, fmt.Errorf("marshal block proposal: %w", err)
	}

	tr.Add(telemetry.Step{
		Step:          1,
		Name:          "Doctor signature attached",
		Algorithm:     "Ed25519",
		InputPreview:  string(TxPayload(block)),
		OutputPreview: previewHex(block.DoctorSignatureHex),
		Detail:        "canonical transaction payload signed by the USB dongle key",
	})
	tr.Add(telemetry.Step{
		Step:          2,
		Name:          "Proposal posted to validator gateway",
		Algorithm:     "HTTP POST " + ValidatePath,
		InputPreview:  string(body),
		OutputPreview: "awaiting countersignature",
	})

	started := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+ValidatePath, bytes.NewReader(body))
	if err != nil {
		return nil, tr, fmt.Errorf("build validator request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		tr.Metric("round_trip_ms", float64(time.Since(started).Microseconds())/1000, "ms")
		return nil, tr, fmt.Errorf("call validator gateway: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, tr, fmt.Errorf("read validator response: %w", err)
	}

	roundTripMS := float64(time.Since(started).Microseconds()) / 1000

	if resp.StatusCode != http.StatusOK {
		return nil, tr, fmt.Errorf("validator gateway returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var envelope ValidationResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, tr, fmt.Errorf("decode validator response: %w", err)
	}

	decision := &Decision{
		Accepted:    envelope.Accepted,
		Checks:      envelope.Checks,
		Reason:      envelope.Reason,
		RoundTripMS: roundTripMS,
	}

	if !envelope.Accepted {
		if decision.Reason == "" {
			decision.Reason = "validator gateway rejected the proposal"
		}
		tr.Metric("round_trip_ms", roundTripMS, "ms")
		return decision, tr, fmt.Errorf("proposal rejected: %s", decision.Reason)
	}

	sealed := envelope.Block
	if sealed.ValidatorSignatureHex == "" {
		return decision, tr, errors.New("validator accepted the block but returned no countersignature")
	}
	if envelope.ValidatorPubkeyHex != "" && !strings.EqualFold(envelope.ValidatorPubkeyHex, sealed.ValidatorPubkeyHex) {
		return decision, tr, errors.New("validator response public key does not match the sealed block")
	}

	decision.Sealed = &sealed

	tr.Add(telemetry.Step{
		Step:      3,
		Name:      "Gateway inspection",
		Algorithm: "three point PoA check",
		Detail:    strings.Join(envelope.Checks, "; "),
	})
	tr.Add(telemetry.Step{
		Step:          4,
		Name:          "Validator countersignature",
		Algorithm:     "Ed25519 over current_hash",
		TagHex:        sealed.ValidatorSignatureHex,
		OutputPreview: sealed.CurrentHash,
		Detail:        "validator public key " + previewHex(sealed.ValidatorPubkeyHex),
	})
	tr.Metric("round_trip_ms", roundTripMS, "ms")
	tr.Metric("checks_passed", float64(len(envelope.Checks)), "")

	return decision, tr.Finish(), nil
}

// previewHex shortens a hex string for the inspector payload.
func previewHex(s string) string {
	if len(s) <= 24 {
		return s
	}
	return s[:24] + "..."
}
