package domain_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

var (
	red   = color.RGBA{R: 255, A: 255}
	green = color.RGBA{G: 255, A: 255}
	blue  = color.RGBA{B: 255, A: 255}
	white = color.RGBA{R: 255, G: 255, B: 255, A: 255}
)

// quadrants returns a w×h image: top-left red, top-right green, bottom-left blue,
// bottom-right white.
func quadrants(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := red
			switch {
			case x >= w/2 && y < h/2:
				c = green
			case x < w/2 && y >= h/2:
				c = blue
			case x >= w/2 && y >= h/2:
				c = white
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// withOrientation inserts an APP1 Exif segment with IFD0 Orientation right after SOI.
func withOrientation(jpg []byte, orientation uint16, bo binary.ByteOrder) []byte {
	var tiff bytes.Buffer
	if bo == binary.LittleEndian {
		tiff.WriteString("II")
	} else {
		tiff.WriteString("MM")
	}
	_ = binary.Write(&tiff, bo, uint16(42))
	_ = binary.Write(&tiff, bo, uint32(8)) // IFD0 offset
	_ = binary.Write(&tiff, bo, uint16(1)) // one entry
	_ = binary.Write(&tiff, bo, uint16(0x0112))
	_ = binary.Write(&tiff, bo, uint16(3)) // SHORT
	_ = binary.Write(&tiff, bo, uint32(1))
	_ = binary.Write(&tiff, bo, orientation)
	_ = binary.Write(&tiff, bo, uint16(0)) // padding of the 4-byte value
	_ = binary.Write(&tiff, bo, uint32(0)) // no next IFD

	seg := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	var out bytes.Buffer
	out.Write(jpg[:2])
	out.Write([]byte{0xFF, 0xE1})
	_ = binary.Write(&out, binary.BigEndian, uint16(len(seg)+2)) //nolint:gosec // small test segment
	out.Write(seg)
	out.Write(jpg[2:])
	return out.Bytes()
}

// corners reports the colors at the centers of the four quadrants of a 512×512 avatar.
func corners(t *testing.T, data []byte) [4]color.RGBA {
	t.Helper()
	img := decodeJPEG(t, data)
	var out [4]color.RGBA
	for i, p := range []image.Point{{128, 128}, {384, 128}, {128, 384}, {384, 384}} {
		r, g, b, _ := img.At(p.X, p.Y).RGBA()
		out[i] = color.RGBA{R: snap(r), G: snap(g), B: snap(b), A: 255}
	}
	return out
}

func snap(v uint32) uint8 {
	if v>>8 > 127 {
		return 255
	}
	return 0
}

// A phone photo is stored as shot and rotated by the EXIF Orientation tag; the
// avatar must look like it does in a viewer that honours the tag.
func TestNormalizeAvatar_ExifOrientation(t *testing.T) {
	// want: top-left, top-right, bottom-left, bottom-right as displayed.
	cases := map[uint16][4]color.RGBA{
		1: {red, green, blue, white},
		2: {green, red, white, blue},
		3: {white, blue, green, red},
		4: {blue, white, red, green},
		5: {red, blue, green, white},
		6: {blue, red, white, green},
		7: {white, green, blue, red},
		8: {green, white, red, blue},
	}
	src := encodeJPEG(t, quadrants(256, 256))
	for o, want := range cases {
		for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
			out, err := domain.NormalizeAvatar(withOrientation(src, o, bo))
			if err != nil {
				t.Fatal(err)
			}
			if got := corners(t, out); got != want {
				t.Errorf("orientation %d (%v): corners %v, want %v", o, bo, got, want)
			}
		}
	}
}

// A landscape 4:3 shot with Orientation=6 is a portrait: the center square of the
// stored pixels is cut out and then rotated.
func TestNormalizeAvatar_ExifOrientationNonSquare(t *testing.T) {
	out, err := domain.NormalizeAvatar(withOrientation(encodeJPEG(t, quadrants(400, 300)), 6, binary.BigEndian))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := corners(t, out), [4]color.RGBA{blue, red, white, green}; got != want {
		t.Fatalf("corners %v, want %v", got, want)
	}
}

// Without the tag, with a broken Exif segment or an invalid value nothing is rotated.
func TestNormalizeAvatar_ExifOrientationIgnored(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, 256))
	broken := append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x0A}, []byte("Exif\x00\x00MM")...)
	broken = append(broken, plain[2:]...)
	for name, data := range map[string][]byte{
		"no exif":        plain,
		"invalid value":  withOrientation(plain, 9, binary.BigEndian),
		"broken segment": broken,
	} {
		out, err := domain.NormalizeAvatar(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got, want := corners(t, out), [4]color.RGBA{red, green, blue, white}; got != want {
			t.Errorf("%s: corners %v, want %v", name, got, want)
		}
	}
}
