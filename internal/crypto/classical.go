package crypto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Classical layer constants. The cipher operates on printable ASCII only, so
// UTF-8 text and control characters survive untouched and the reversible
// pipeline never corrupts Indonesian characters.
const (
	printableLow  = 32                               // space
	printableHigh = 126                              // tilde
	printableSpan = printableHigh - printableLow + 1 // 95
)

// ClassicalCipher is the Layer 1 primitive: an extended Vigenere over printable
// ASCII followed by a keyed columnar transposition.
type ClassicalCipher struct {
	vigenereKey      []byte
	transpositionKey string
}

// NewClassicalCipher validates both keys.
func NewClassicalCipher(vigenereKey, transpositionKey string) (*ClassicalCipher, error) {
	if len(vigenereKey) == 0 {
		return nil, errors.New("vigenere key must not be empty")
	}
	if len(transpositionKey) == 0 {
		return nil, errors.New("transposition key must not be empty")
	}
	if strings.Contains(transpositionKey, "|") {
		return nil, errors.New("transposition key must not contain the pipe character")
	}

	normalised := make([]byte, 0, len(vigenereKey))
	for i := 0; i < len(vigenereKey); i++ {
		b := vigenereKey[i]
		if b < printableLow || b > printableHigh {
			// Fold non printable key bytes into the printable range so the shift
			// arithmetic stays inside the alphabet.
			b = printableLow + (b % printableSpan)
		}
		normalised = append(normalised, b)
	}

	return &ClassicalCipher{
		vigenereKey:      normalised,
		transpositionKey: transpositionKey,
	}, nil
}

// Encrypt applies Vigenere then columnar transposition.
func (c *ClassicalCipher) Encrypt(plaintext string) (string, error) {
	vig := c.vigenere(plaintext, false)
	return c.transpose(vig)
}

// Decrypt reverses the transposition then the Vigenere.
func (c *ClassicalCipher) Decrypt(ciphertext string) (string, error) {
	vig, err := c.untranspose(ciphertext)
	if err != nil {
		return "", err
	}
	return c.vigenere(vig, true), nil
}

// vigenere shifts printable ASCII by the corresponding key byte. Bytes outside
// the printable range are copied verbatim.
func (c *ClassicalCipher) vigenere(input string, inverse bool) string {
	out := make([]byte, len(input))
	keyLen := len(c.vigenereKey)
	keyPos := 0

	for i := 0; i < len(input); i++ {
		b := input[i]
		if b < printableLow || b > printableHigh {
			out[i] = b
			continue
		}
		shift := int(c.vigenereKey[keyPos%keyLen]) - printableLow
		value := int(b) - printableLow
		if inverse {
			value = (value - shift + printableSpan*2) % printableSpan
		} else {
			value = (value + shift) % printableSpan
		}
		out[i] = byte(value + printableLow)
		keyPos++
	}
	return string(out)
}

// transpose arranges the Vigenere output into a grid of width len(key), reads
// columns in key-sorted order, and prefixes the original length so padding can
// be stripped on the way back.
func (c *ClassicalCipher) transpose(input string) (string, error) {
	width := len(c.transpositionKey)
	data := []byte(input)

	// Header carries the true payload length as 4 big endian bytes rendered in
	// printable ASCII so the whole intermediate value stays readable in the UI.
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	padded := len(data)
	if rem := padded % width; rem != 0 {
		padded += width - rem
	}

	buf := make([]byte, padded)
	copy(buf, data)
	for i := len(data); i < padded; i++ {
		buf[i] = ' '
	}

	rows := padded / width
	order := c.columnOrder()

	out := make([]byte, 0, padded+4)
	out = append(out, header...)
	for _, col := range order {
		for row := 0; row < rows; row++ {
			out = append(out, buf[row*width+col])
		}
	}
	return string(out), nil
}

// untranspose reverses transpose.
func (c *ClassicalCipher) untranspose(input string) (string, error) {
	if len(input) < 4 {
		return "", errors.New("classical ciphertext is truncated")
	}
	width := len(c.transpositionKey)
	payloadLen := int(binary.BigEndian.Uint32([]byte(input[:4])))
	body := []byte(input[4:])

	if payloadLen < 0 {
		return "", errors.New("classical ciphertext header is invalid")
	}
	if len(body)%width != 0 {
		return "", fmt.Errorf("classical ciphertext length %d is not a multiple of key width %d", len(body), width)
	}
	if payloadLen > len(body) {
		return "", errors.New("classical ciphertext header exceeds body length")
	}

	rows := len(body) / width
	buf := make([]byte, len(body))
	order := c.columnOrder()

	idx := 0
	for _, col := range order {
		for row := 0; row < rows; row++ {
			buf[row*width+col] = body[idx]
			idx++
		}
	}

	return string(buf[:payloadLen]), nil
}

// columnOrder returns column indices sorted by their key byte, breaking ties by
// the original position so the mapping is deterministic and invertible.
func (c *ClassicalCipher) columnOrder() []int {
	width := len(c.transpositionKey)
	order := make([]int, width)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return c.transpositionKey[order[a]] < c.transpositionKey[order[b]]
	})
	return order
}
