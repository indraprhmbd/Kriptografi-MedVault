// Package biometric verifies human presence without native dependencies.
//
// Two mechanisms are supported, matching DEC-05:
//   - A 64 bit difference hash (dHash) computed in pure Go from a webcam
//     snapshot, compared by Hamming distance.
//   - WebAuthn platform authenticator assertions (Windows Hello), handled in
//     the sibling webauthn.go file.
package biometric

import (
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"math"
	"math/bits"
	"strings"
)

// dHash grid dimensions. A 9 by 8 grayscale grid yields 8 horizontal
// comparisons per row across 8 rows, which is exactly 64 bits.
const (
	dhashWidth  = 9
	dhashHeight = 8
)

// VerifyResult describes a dHash comparison.
type VerifyResult struct {
	Match      bool    `json:"match"`
	Distance   int     `json:"distance"`
	Similarity float64 `json:"similarity_percent"`
	Threshold  int     `json:"threshold"`
}

// ComputeDHash reduces an image to a 64 bit difference hash.
//
// The image is converted to luminance, box averaged down to a 9 by 8 grid, and
// each cell is compared with its right neighbour. The result is invariant to
// brightness shifts and mild scaling, which is what makes it usable for a live
// webcam snapshot matched against an enrolment photo.
func ComputeDHash(img image.Image) (uint64, error) {
	if img == nil {
		return 0, errors.New("image is nil")
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0, errors.New("image has no pixels")
	}

	grid := make([]float64, dhashWidth*dhashHeight)

	// Box average each source cell into the target grid. Sampling every pixel
	// keeps the result stable under resizing and avoids aliasing from a naive
	// nearest neighbour lookup.
	for gy := 0; gy < dhashHeight; gy++ {
		y0 := bounds.Min.Y + gy*height/dhashHeight
		y1 := bounds.Min.Y + (gy+1)*height/dhashHeight
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for gx := 0; gx < dhashWidth; gx++ {
			x0 := bounds.Min.X + gx*width/dhashWidth
			x1 := bounds.Min.X + (gx+1)*width/dhashWidth
			if x1 <= x0 {
				x1 = x0 + 1
			}

			var sum float64
			var count int
			for y := y0; y < y1 && y < bounds.Max.Y; y++ {
				for x := x0; x < x1 && x < bounds.Max.X; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					// RGBA returns 16 bit values, so scale to 8 bit luminance.
					lum := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
					sum += lum
					count++
				}
			}
			if count == 0 {
				grid[gy*dhashWidth+gx] = 0
				continue
			}
			grid[gy*dhashWidth+gx] = sum / float64(count)
		}
	}

	var hash uint64
	bit := 0
	for gy := 0; gy < dhashHeight; gy++ {
		for gx := 0; gx < dhashWidth-1; gx++ {
			if grid[gy*dhashWidth+gx] > grid[gy*dhashWidth+gx+1] {
				hash |= 1 << uint(63-bit)
			}
			bit++
		}
	}

	return hash, nil
}

// HammingDistance counts differing bits between two hashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// Similarity converts a Hamming distance into a percentage of matching bits.
func Similarity(a, b uint64) float64 {
	distance := HammingDistance(a, b)
	return (64.0 - float64(distance)) / 64.0 * 100.0
}

// HashHex renders a hash as 16 lowercase hex characters.
func HashHex(h uint64) string {
	return fmt.Sprintf("%016x", h)
}

// ParseHashHex parses a 16 character hex hash.
func ParseHashHex(s string) (uint64, error) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) != 16 {
		return 0, fmt.Errorf("dHash must be 16 hex characters, got %d", len(trimmed))
	}
	raw, err := hex.DecodeString(trimmed)
	if err != nil {
		return 0, fmt.Errorf("dHash is not valid hex: %w", err)
	}

	var h uint64
	for _, b := range raw {
		h = h<<8 | uint64(b)
	}
	return h, nil
}

// VerifyHashes compares two hex hashes against a maximum distance threshold.
func VerifyHashes(enrolledHex, liveHex string, maxDistance int) (*VerifyResult, error) {
	if maxDistance < 0 || maxDistance > 64 {
		return nil, fmt.Errorf("max distance must be between 0 and 64, got %d", maxDistance)
	}

	enrolled, err := ParseHashHex(enrolledHex)
	if err != nil {
		return nil, fmt.Errorf("enrolled hash: %w", err)
	}
	live, err := ParseHashHex(liveHex)
	if err != nil {
		return nil, fmt.Errorf("live hash: %w", err)
	}

	distance := HammingDistance(enrolled, live)
	return &VerifyResult{
		Match:      distance <= maxDistance,
		Distance:   distance,
		Similarity: (64.0 - float64(distance)) / 64.0 * 100.0,
		Threshold:  maxDistance,
	}, nil
}

// RoundSimilarity trims floating point noise for display.
func RoundSimilarity(v float64) float64 {
	return math.Round(v*10) / 10
}
