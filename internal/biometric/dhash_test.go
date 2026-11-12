package biometric

import (
	"image"
	"image/color"
	"math/rand"
	"strings"
	"testing"
)

// makeGradient builds a deterministic grayscale gradient of the given size.
func makeGradient(width, height int, offset uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v := uint8((x*3 + y*5) % 200)
			v = uint8(int(v) + int(offset))
			if offset > 0 && v > 255 {
				v = 255
			}
			img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}

func TestComputeDHashIsDeterministic(t *testing.T) {
	img := makeGradient(64, 64, 0)

	first, err := ComputeDHash(img)
	if err != nil {
		t.Fatalf("ComputeDHash: %v", err)
	}
	second, err := ComputeDHash(img)
	if err != nil {
		t.Fatalf("ComputeDHash: %v", err)
	}
	if first != second {
		t.Fatalf("dHash must be deterministic: %x vs %x", first, second)
	}
}

func TestIdenticalImagesHaveZeroDistance(t *testing.T) {
	img := makeGradient(64, 64, 0)

	a, err := ComputeDHash(img)
	if err != nil {
		t.Fatalf("ComputeDHash: %v", err)
	}
	b, err := ComputeDHash(img)
	if err != nil {
		t.Fatalf("ComputeDHash: %v", err)
	}

	if d := HammingDistance(a, b); d != 0 {
		t.Fatalf("identical images must have distance 0, got %d", d)
	}
}

func TestBrightnessVariationStaysWithinThreshold(t *testing.T) {
	const maxDistance = 14

	reference, err := ComputeDHash(makeGradient(64, 64, 0))
	if err != nil {
		t.Fatalf("ComputeDHash reference: %v", err)
	}

	// Small brightness and contrast changes must not break verification,
	// because lighting differs between enrolment and login.
	for _, offset := range []uint8{2, 5, 10} {
		live, err := ComputeDHash(makeGradient(64, 64, offset))
		if err != nil {
			t.Fatalf("ComputeDHash offset %d: %v", offset, err)
		}
		if d := HammingDistance(reference, live); d > maxDistance {
			t.Fatalf("offset %d produced distance %d, expected <= %d", offset, d, maxDistance)
		}
	}
}

func TestUnrelatedImagesExceedThreshold(t *testing.T) {
	a, err := ComputeDHash(makeGradient(64, 64, 0))
	if err != nil {
		t.Fatalf("ComputeDHash a: %v", err)
	}

	// An inverse gradient has the opposite local ordering almost everywhere.
	inverse := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			v := uint8(255 - ((x*3 + y*5) % 200))
			inverse.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}

	b, err := ComputeDHash(inverse)
	if err != nil {
		t.Fatalf("ComputeDHash b: %v", err)
	}

	if d := HammingDistance(a, b); d <= 14 {
		t.Fatalf("inverted pattern should exceed the threshold, got distance %d", d)
	}
}

func TestHammingDistanceBounds(t *testing.T) {
	cases := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{0, 1, 1},
		{0xFFFFFFFFFFFFFFFF, 0, 64},
		{0xF0F0F0F0F0F0F0F0, 0x0F0F0F0F0F0F0F0F, 64},
	}

	for _, c := range cases {
		if got := HammingDistance(c.a, c.b); got != c.want {
			t.Fatalf("HammingDistance(%x, %x) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := HammingDistance(c.b, c.a); got != c.want {
			t.Fatalf("HammingDistance must be symmetric")
		}
	}
}

func TestSimilarityPercentage(t *testing.T) {
	if got := Similarity(0, 0); got != 100.0 {
		t.Fatalf("identical hashes must be 100 percent, got %f", got)
	}
	if got := Similarity(0, 0xFFFFFFFFFFFFFFFF); got != 0.0 {
		t.Fatalf("opposite hashes must be 0 percent, got %f", got)
	}
	if got := Similarity(0, 0x7F); got < 89.0 || got > 90.0 {
		t.Fatalf("7 bit difference should be about 89 percent, got %f", got)
	}
}

func TestHexRoundTrip(t *testing.T) {
	const h uint64 = 0xDEADBEEFCAFEF00D

	encoded := HashHex(h)
	if len(encoded) != 16 {
		t.Fatalf("hex encoding must be 16 characters, got %d", len(encoded))
	}
	if encoded != strings.ToLower(encoded) {
		t.Fatal("hex encoding must be lowercase")
	}

	decoded, err := ParseHashHex(encoded)
	if err != nil {
		t.Fatalf("ParseHashHex: %v", err)
	}
	if decoded != h {
		t.Fatalf("round trip mismatch: got %x want %x", decoded, h)
	}
}

func TestParseHashHexRejectsInvalid(t *testing.T) {
	for _, bad := range []string{"", "zz", "0f0f", strings.Repeat("f", 17)} {
		if _, err := ParseHashHex(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestVerifyDHASHMatchesWithinThreshold(t *testing.T) {
	enrolled := makeGradient(64, 64, 0)
	live := makeGradient(64, 64, 4)

	enrolledHash, err := ComputeDHash(enrolled)
	if err != nil {
		t.Fatalf("enrolled hash: %v", err)
	}
	liveHash, err := ComputeDHash(live)
	if err != nil {
		t.Fatalf("live hash: %v", err)
	}

	result, err := VerifyHashes(HashHex(enrolledHash), HashHex(liveHash), 14)
	if err != nil {
		t.Fatalf("VerifyHashes: %v", err)
	}
	if !result.Match {
		t.Fatalf("expected a match, distance was %d", result.Distance)
	}
	if result.Similarity < 78.0 {
		t.Fatalf("expected similarity above 78 percent, got %f", result.Similarity)
	}
}

func TestVerifyDHASHRejectsOutsider(t *testing.T) {
	enrolled := HashHex(0x0000000000000000)
	live := HashHex(0xFFFFFFFFFFFFFFFF)

	result, err := VerifyHashes(enrolled, live, 14)
	if err != nil {
		t.Fatalf("VerifyHashes: %v", err)
	}
	if result.Match {
		t.Fatal("completely different hashes must not match")
	}
	if result.Distance != 64 {
		t.Fatalf("expected distance 64, got %d", result.Distance)
	}
}

func TestComputeDHashRejectsEmptyImage(t *testing.T) {
	if _, err := ComputeDHash(image.NewNRGBA(image.Rect(0, 0, 0, 0))); err == nil {
		t.Fatal("expected error for a zero sized image")
	}
}

func TestComputeDHashAcceptsArbitraryInputSize(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	img := image.NewNRGBA(image.Rect(0, 0, 37, 91))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
	}

	if _, err := ComputeDHash(img); err != nil {
		t.Fatalf("ComputeDHash must accept any size: %v", err)
	}
}

func TestComputeDHashIsStableAcrossScaling(t *testing.T) {
	small, err := ComputeDHash(makeGradient(64, 64, 0))
	if err != nil {
		t.Fatalf("small: %v", err)
	}
	large, err := ComputeDHash(makeGradient(128, 128, 0))
	if err != nil {
		t.Fatalf("large: %v", err)
	}

	// The same structural pattern at a different resolution should hash close.
	// A synthetic linear gradient is harsher than a real face because the
	// quantization boundaries shift with the grid size, so the bound is looser
	// than the 14 bit login threshold.
	if d := HammingDistance(small, large); d > 24 {
		t.Fatalf("scale change produced distance %d, expected <= 24", d)
	}
}
