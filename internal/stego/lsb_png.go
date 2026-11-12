// Package stego implements authenticated LSB steganography for lossless PNG
// medical scans.
//
// A payload is compressed with zlib, authenticated with HMAC-SHA256, and packed
// into a fixed binary envelope. The envelope bits are written into the least
// significant bit of the red, green, and blue channels. The alpha channel is
// never modified, which keeps transparency intact.
//
// Extraction verifies the HMAC before decompressing, so a modified image is
// rejected rather than silently producing corrupted metadata.
package stego

import (
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"

	"medvault/internal/telemetry"
)

// Magic header that identifies a MedVault stego packet.
var magic = [4]byte{'M', 'V', 'S', 'T'}

// Envelope sizes in bytes.
const (
	magicSize  = 4
	lengthSize = 4
	hmacSize   = sha256.Size
	headerSize = magicSize + lengthSize + hmacSize // 40 bytes
)

// ChannelsCarryingBits is the number of colour channels used per pixel.
const ChannelsCarryingBits = 3

// Payload is the metadata hidden inside the image.
type Payload struct {
	MRN              string `json:"mrn"`
	PatientName      string `json:"patient_name"`
	Doctor           string `json:"doctor"`
	DiagnosisSummary string `json:"diagnosis_summary"`
	Timestamp        string `json:"timestamp"`
}

// EmbedResult carries the stego image and the quality telemetry.
type EmbedResult struct {
	PNG          []byte
	Trace        *telemetry.Trace
	PSNR         float64
	MSE          float64
	CapacityBits int
	UsedBits     int
	HMACTagHex   string
}

// ExtractResult carries the recovered payload and verification telemetry.
type ExtractResult struct {
	Payload      Payload
	Trace        *telemetry.Trace
	HMACVerified bool
	UsedBits     int
}

// Embed hides payload inside src and returns the stego PNG.
func Embed(src []byte, payload Payload, hmacKey []byte) (*EmbedResult, error) {
	if len(hmacKey) == 0 {
		return nil, errors.New("stego HMAC key must not be empty")
	}

	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode source image as PNG: %w", err)
	}

	orig := toNRGBA(img)
	bounds := orig.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width == 0 || height == 0 {
		return nil, errors.New("source image is empty")
	}

	capacityBits := width * height * ChannelsCarryingBits

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(jsonBytes); err != nil {
		return nil, fmt.Errorf("compress payload: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close compressor: %w", err)
	}
	compressedBytes := compressed.Bytes()

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(compressedBytes)
	tag := mac.Sum(nil)

	packet := make([]byte, 0, headerSize+len(compressedBytes))
	packet = append(packet, magic[:]...)
	lenBuf := make([]byte, lengthSize)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(compressedBytes)))
	packet = append(packet, lenBuf...)
	packet = append(packet, tag...)
	packet = append(packet, compressedBytes...)

	totalBits := len(packet) * 8
	if totalBits > capacityBits {
		return nil, fmt.Errorf(
			"image capacity exceeded: payload needs %d bits, image holds %d bits",
			totalBits, capacityBits,
		)
	}

	out := cloneNRGBA(orig)
	writeBits(out, packet)

	mse, psnr := quality(orig, out)

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, out); err != nil {
		return nil, fmt.Errorf("encode stego PNG: %w", err)
	}

	tr := telemetry.NewTrace("LSB_PNG_STEGO_EMBED")
	tr.Add(telemetry.Step{
		Step:          1,
		Name:          "Payload compression",
		Algorithm:     "zlib plus JSON",
		InputPreview:  telemetry.Preview(string(jsonBytes), 96),
		OutputPreview: fmt.Sprintf("%d compressed bytes", len(compressedBytes)),
	})
	tr.Add(telemetry.Step{
		Step:      2,
		Name:      "HMAC-SHA256 authentication tag",
		Algorithm: "HMAC-SHA256",
		TagHex:    fmt.Sprintf("%x", tag),
		Detail:    "tag covers the compressed payload only",
	})
	tr.Add(telemetry.Step{
		Step:      3,
		Name:      "LSB embedding into RGB channels",
		Algorithm: "1 bit per channel, alpha untouched",
		Detail:    fmt.Sprintf("%d bits written across %d pixels", totalBits, width*height),
	})
	tr.Add(telemetry.Step{
		Step:          4,
		Name:          "Lossless re-encode",
		Algorithm:     "image/png",
		OutputPreview: fmt.Sprintf("%d bytes", encoded.Len()),
	})
	tr.Metric("psnr_db", psnr, "dB")
	tr.Metric("mse", mse, "")
	tr.Metric("capacity_bits", float64(capacityBits), "bits")
	tr.Metric("used_bits", float64(totalBits), "bits")
	tr.Metric("utilization_percent", float64(totalBits)/float64(capacityBits)*100, "%")

	return &EmbedResult{
		PNG:          encoded.Bytes(),
		Trace:        tr.Finish(),
		PSNR:         psnr,
		MSE:          mse,
		CapacityBits: capacityBits,
		UsedBits:     totalBits,
		HMACTagHex:   fmt.Sprintf("%x", tag),
	}, nil
}

// Extract recovers and authenticates the payload hidden in src.
func Extract(src []byte, hmacKey []byte) (*ExtractResult, error) {
	if len(hmacKey) == 0 {
		return nil, errors.New("stego HMAC key must not be empty")
	}

	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image as PNG: %w", err)
	}

	nrgba := toNRGBA(img)
	bounds := nrgba.Bounds()
	capacityBits := bounds.Dx() * bounds.Dy() * ChannelsCarryingBits

	if capacityBits < headerSize*8 {
		return nil, errors.New("image is too small to contain a MedVault watermark")
	}

	headerBits := headerSize * 8
	header := readBits(nrgba, 0, headerBits)

	if !bytes.Equal(header[:magicSize], magic[:]) {
		return nil, errors.New("image does not contain a MedVault steganography watermark")
	}

	payloadLen := int(binary.BigEndian.Uint32(header[magicSize : magicSize+lengthSize]))
	if payloadLen <= 0 {
		return nil, errors.New("watermark payload length is invalid")
	}

	totalBits := (headerSize + payloadLen) * 8
	if totalBits > capacityBits {
		return nil, fmt.Errorf(
			"image capacity exceeded: watermark claims %d bits, image holds %d bits",
			totalBits, capacityBits,
		)
	}

	expectedTag := header[magicSize+lengthSize : headerSize]
	body := readBits(nrgba, headerBits, payloadLen*8)

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(body)
	computedTag := mac.Sum(nil)

	if !hmac.Equal(computedTag, expectedTag) {
		return nil, errors.New("watermark HMAC verification failed, the image was modified or the key is wrong")
	}

	zr, err := zlib.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("open compressed watermark payload: %w", err)
	}
	defer zr.Close()

	plain, err := io.ReadAll(io.LimitReader(zr, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("decompress watermark payload: %w", err)
	}

	var payload Payload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, fmt.Errorf("parse watermark payload: %w", err)
	}

	verified := true
	tr := telemetry.NewTrace("LSB_PNG_STEGO_EXTRACT")
	tr.Add(telemetry.Step{
		Step:      1,
		Name:      "Packet header read",
		Algorithm: "LSB read, 320 bits",
		Detail:    fmt.Sprintf("magic MVST, payload length %d bytes", payloadLen),
	})
	tr.Add(telemetry.Step{
		Step:      2,
		Name:      "HMAC-SHA256 verification",
		Algorithm: "HMAC-SHA256",
		TagHex:    fmt.Sprintf("%x", computedTag),
		Verified:  &verified,
	})
	tr.Add(telemetry.Step{
		Step:          3,
		Name:          "zlib decompression and JSON parse",
		Algorithm:     "compress/zlib plus encoding/json",
		OutputPreview: telemetry.Preview(string(plain), 96),
	})
	tr.Metric("used_bits", float64(totalBits), "bits")

	return &ExtractResult{
		Payload:      payload,
		Trace:        tr.Finish(),
		HMACVerified: true,
		UsedBits:     totalBits,
	}, nil
}

// writeBits writes packet bits sequentially into the RGB channels.
func writeBits(img *image.NRGBA, packet []byte) {
	bitIndex := 0
	total := len(packet) * 8
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y && bitIndex < total; y++ {
		for x := bounds.Min.X; x < bounds.Max.X && bitIndex < total; x++ {
			offset := img.PixOffset(x, y)
			// Pix is R,G,B,A in NRGBA order.
			for channel := 0; channel < ChannelsCarryingBits && bitIndex < total; channel++ {
				bit := (packet[bitIndex/8] >> (7 - uint(bitIndex%8))) & 1
				img.Pix[offset+channel] = (img.Pix[offset+channel] & 0xFE) | bit
				bitIndex++
			}
		}
	}
}

// readBits reads n bits starting at startBit, most significant bit first.
//
// The bit stream is laid out exactly like writeBits produces it: pixel 0
// channel R, pixel 0 channel G, pixel 0 channel B, pixel 1 channel R, and so
// on, traversing rows left to right.
func readBits(img *image.NRGBA, startBit, n int) []byte {
	out := make([]byte, (n+7)/8)
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	totalBits := width * height * ChannelsCarryingBits

	for i := 0; i < n; i++ {
		bitPos := startBit + i
		if bitPos >= totalBits {
			break
		}
		pixel := bitPos / ChannelsCarryingBits
		channel := bitPos % ChannelsCarryingBits
		x := bounds.Min.X + pixel%width
		y := bounds.Min.Y + pixel/width
		offset := img.PixOffset(x, y)

		bit := img.Pix[offset+channel] & 1
		out[i/8] |= bit << (7 - uint(i%8))
	}
	return out
}

// quality computes MSE and PSNR over the RGB channels only.
func quality(a, b *image.NRGBA) (float64, float64) {
	bounds := a.Bounds()
	var sum float64
	var count int

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ao := a.PixOffset(x, y)
			bo := b.PixOffset(x, y)
			for channel := 0; channel < ChannelsCarryingBits; channel++ {
				diff := float64(int(a.Pix[ao+channel]) - int(b.Pix[bo+channel]))
				sum += diff * diff
				count++
			}
		}
	}

	if count == 0 {
		return 0, 0
	}

	mse := sum / float64(count)
	if mse == 0 {
		return 0, math.Inf(1)
	}
	psnr := 10 * math.Log10((255*255)/mse)
	return mse, psnr
}

// toNRGBA converts any decoded image into an NRGBA copy.
func toNRGBA(src image.Image) *image.NRGBA {
	if nrgba, ok := src.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

// cloneNRGBA returns an independent copy so the caller's image is untouched.
func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}
