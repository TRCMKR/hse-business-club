package domain_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// gradient returns a w×h image with a horizontal red and vertical green gradient,
// so that crops and scales are observable in pixel values.
func gradient(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / max(w-1, 1)), G: uint8(y * 255 / max(h-1, 1)), B: 128, A: 255})
		}
	}
	return img
}

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pngHeaderOnly returns a PNG signature with an IHDR chunk of the given size and
// a truncated IDAT: DecodeConfig succeeds, a full decode would fail.
func pngHeaderOnly(w, h uint32) []byte {
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // RGBA
	writeChunk(&buf, "IHDR", ihdr)
	writeChunk(&buf, "IDAT", []byte{0x78, 0x9c})
	return buf.Bytes()
}

func writeChunk(buf *bytes.Buffer, typ string, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(data)))
	crc := crc32.NewIEEE()
	_, _ = crc.Write([]byte(typ))
	_, _ = crc.Write(data)
	buf.WriteString(typ)
	buf.Write(data)
	_ = binary.Write(buf, binary.BigEndian, crc.Sum32())
}

// solidWebP encodes a w×h single-color lossless WebP (VP8L). Every prefix code has
// a single symbol, so pixels take zero bits and the file is a few dozen bytes for
// any size. The standard library has no WebP encoder; this keeps tests fixture-free.
func solidWebP(w, h int, c color.NRGBA) []byte {
	var bw bitWriter
	bw.write(0x2f, 8) // VP8L signature
	bw.write(uint32(w-1), 14)
	bw.write(uint32(h-1), 14)
	bw.write(0, 1) // alpha is used
	bw.write(0, 3) // version
	bw.write(0, 1) // no transforms
	bw.write(0, 1) // no color cache
	bw.write(0, 1) // no meta prefix codes
	for _, sym := range []uint8{c.G, c.R, c.B, c.A} {
		bw.write(1, 1) // simple code
		bw.write(0, 1) // one symbol
		bw.write(1, 1) // 8-bit symbol
		bw.write(uint32(sym), 8)
	}
	bw.write(1, 1) // distance: simple code
	bw.write(0, 1) // one symbol
	bw.write(0, 1) // 1-bit symbol
	bw.write(0, 1) // symbol 0
	data := bw.bytes()

	var buf bytes.Buffer
	chunk := len(data)
	padded := chunk + chunk%2
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(4+8+padded))
	buf.WriteString("WEBPVP8L")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(chunk))
	buf.Write(data)
	if chunk%2 == 1 {
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

// bitWriter writes bits LSB-first, as VP8L expects.
type bitWriter struct {
	buf   []byte
	acc   uint64
	nbits uint
}

func (b *bitWriter) write(v uint32, n uint) {
	b.acc |= uint64(v) << b.nbits
	b.nbits += n
	for b.nbits >= 8 {
		b.buf = append(b.buf, byte(b.acc))
		b.acc >>= 8
		b.nbits -= 8
	}
}

func (b *bitWriter) bytes() []byte {
	if b.nbits > 0 {
		return append(b.buf, byte(b.acc))
	}
	return b.buf
}
