package stego

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// testHMACKey is a deterministic 32 byte key so tests are reproducible.
func testHMACKey() []byte { return []byte("0123456789abcdef0123456789abcdef") }

// makeTestPNG builds a deterministic gradient PNG of the requested size.
func makeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 3) % 256),
				G: uint8((y * 5) % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func testPayload() Payload {
	return Payload{
		MRN:              "RM-2026-0001",
		PatientName:      "Budi Santoso",
		Doctor:           "dr. Andi",
		DiagnosisSummary: "Hipertensi Stage 2, TD 160/100 mmHg",
		Timestamp:        "2026-11-12T10:00:00Z",
	}
}

func TestEmbedAndExtractRoundTrip(t *testing.T) {
	src := makeTestPNG(t, 300, 300)
	payload := testPayload()

	result, err := Embed(src, payload, testHMACKey())
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(result.PNG) == 0 {
		t.Fatal("embed must return PNG bytes")
	}
	if result.Trace == nil {
		t.Fatal("embed must return a trace")
	}

	res, err := Extract(result.PNG, testHMACKey())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Trace == nil {
		t.Fatal("extract must return a trace")
	}

	if res.Payload.MRN != payload.MRN {
		t.Fatalf("mrn mismatch: got %q want %q", res.Payload.MRN, payload.MRN)
	}
	if res.Payload.PatientName != payload.PatientName {
		t.Fatalf("patient mismatch: got %q want %q", res.Payload.PatientName, payload.PatientName)
	}
	if res.Payload.DiagnosisSummary != payload.DiagnosisSummary {
		t.Fatalf("diagnosis mismatch: got %q want %q", res.Payload.DiagnosisSummary, payload.DiagnosisSummary)
	}
}

func TestEmbedKeepsPSNRAboveThreshold(t *testing.T) {
	src := makeTestPNG(t, 300, 300)

	result, err := Embed(src, testPayload(), testHMACKey())
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if result.PSNR <= 50.0 {
		t.Fatalf("PSNR must exceed 50 dB for 1 LSB embedding, got %.2f", result.PSNR)
	}
	if result.MSE <= 0 {
		t.Fatalf("MSE must be positive when bits are modified, got %f", result.MSE)
	}
}

func TestExtractRejectsTamperedImage(t *testing.T) {
	src := makeTestPNG(t, 300, 300)

	result, err := Embed(src, testPayload(), testHMACKey())
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(result.PNG))
	if err != nil {
		t.Fatalf("decode stego png: %v", err)
	}

	// Flip the least significant bit of the final bit of the payload body so
	// the magic header and length stay valid and only the HMAC can catch it.
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		nrgba = toNRGBA(img)
	}
	bitPos := result.UsedBits - 1
	pixel := bitPos / 3
	channel := bitPos % 3
	x, y := pixel%300, pixel/300

	off := nrgba.PixOffset(x, y)
	nrgba.Pix[off+channel] ^= 0x01

	var tampered bytes.Buffer
	if err := png.Encode(&tampered, nrgba); err != nil {
		t.Fatalf("re-encode tampered png: %v", err)
	}

	if _, err := Extract(tampered.Bytes(), testHMACKey()); err == nil {
		t.Fatal("extract must fail when the HMAC tag no longer matches")
	}
}

func TestExtractRejectsWrongHMACKey(t *testing.T) {
	src := makeTestPNG(t, 300, 300)

	result, err := Embed(src, testPayload(), testHMACKey())
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	wrong := []byte("ffffffffffffffffffffffffffffffff")
	if _, err := Extract(result.PNG, wrong); err == nil {
		t.Fatal("extract must fail with a different HMAC key")
	}
}

func TestExtractRejectsImageWithoutWatermark(t *testing.T) {
	plain := makeTestPNG(t, 300, 300)

	_, err := Extract(plain, testHMACKey())
	if err == nil {
		t.Fatal("expected an error for an image without a MedVault watermark")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "watermark") {
		t.Fatalf("error should mention the missing watermark, got: %v", err)
	}
}

func TestEmbedRejectsInsufficientCapacity(t *testing.T) {
	// 8x8 pixels gives 192 bits, far below the 320 bit header alone.
	tiny := makeTestPNG(t, 8, 8)

	_, err := Embed(tiny, testPayload(), testHMACKey())
	if err == nil {
		t.Fatal("expected a capacity error for a tiny image")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "capacity") {
		t.Fatalf("error should mention capacity, got: %v", err)
	}
}

func TestEmbedRejectsNonPNGInput(t *testing.T) {
	if _, err := Embed([]byte("not an image"), testPayload(), testHMACKey()); err == nil {
		t.Fatal("expected an error for non image input")
	}
}

func TestEmbedRejectsEmptyHMACKey(t *testing.T) {
	src := makeTestPNG(t, 64, 64)
	if _, err := Embed(src, testPayload(), nil); err == nil {
		t.Fatal("expected an error for an empty HMAC key")
	}
}

func TestEmbedProducesValidPNG(t *testing.T) {
	src := makeTestPNG(t, 120, 120)

	result, err := Embed(src, testPayload(), testHMACKey())
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(result.PNG))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 120 || b.Dy() != 120 {
		t.Fatalf("dimensions changed: %dx%d", b.Dx(), b.Dy())
	}
}

func TestAlphaChannelIsPreserved(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}

	result, err := Embed(buf.Bytes(), testPayload(), testHMACKey())
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	out, err := png.Decode(bytes.NewReader(result.PNG))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	outNRGBA := toNRGBA(out)
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if got := outNRGBA.NRGBAAt(x, y).A; got != 255 {
				t.Fatalf("alpha modified at %d,%d: got %d", x, y, got)
			}
		}
	}
}

func TestPayloadJSONIsStable(t *testing.T) {
	raw, err := json.Marshal(testPayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"mrn"`)) {
		t.Fatalf("payload must serialise the mrn field, got %s", raw)
	}
}
