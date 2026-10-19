package domain_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand/v2"
	"runtime"
	"testing"
	"time"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	return img
}

func assertAvatarErr(t *testing.T, err error, msg string) {
	t.Helper()
	var e *apperr.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *apperr.Error %q, got %v", msg, err)
	}
	if e.Kind != apperr.KindValidation || e.Field != domain.FieldAvatar || e.Msg != msg {
		t.Fatalf("want validation avatar %q, got %+v", msg, e)
	}
}

func TestNormalizeAvatar_Formats(t *testing.T) {
	cases := map[string][]byte{
		"png":  encodePNG(t, gradient(300, 200)),
		"jpeg": encodeJPEG(t, gradient(200, 300)),
		"webp": solidWebP(100, 80, color.NRGBA{R: 10, G: 200, B: 30, A: 255}),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := domain.NormalizeAvatar(data)
			if err != nil {
				t.Fatal(err)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
			if err != nil || format != "jpeg" {
				t.Fatalf("format %q, err %v", format, err)
			}
			if cfg.Width != domain.AvatarSize || cfg.Height != domain.AvatarSize {
				t.Fatalf("size %dx%d", cfg.Width, cfg.Height)
			}
		})
	}
}

func TestNormalizeAvatar_WebPColor(t *testing.T) {
	out, err := domain.NormalizeAvatar(solidWebP(64, 64, color.NRGBA{R: 10, G: 200, B: 30, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decodeJPEG(t, out).At(256, 256).RGBA()
	if near(r>>8, 10) && near(g>>8, 200) && near(b>>8, 30) {
		return
	}
	t.Fatalf("center pixel %d,%d,%d, want ~10,200,30", r>>8, g>>8, b>>8)
}

func near(got uint32, want int) bool {
	d := int(got) - want
	return d >= -8 && d <= 8
}

// A landscape image is cropped to its centered square: the left and right edges of
// the output must be the source columns at (w-h)/2 and (w+h)/2, not 0 and w.
func TestNormalizeAvatar_CenterSquare(t *testing.T) {
	src := gradient(1200, 600) // red = x*255/1199
	out, err := domain.NormalizeAvatar(encodePNG(t, src))
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEG(t, out)
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("bounds %v", b)
	}
	left, _, _, _ := img.At(2, 256).RGBA()
	right, _, _, _ := img.At(509, 256).RGBA()
	// crop is x ∈ [300, 900): red ≈ 64 … 191
	if !near(left>>8, 64) || !near(right>>8, 191) {
		t.Fatalf("edge reds %d / %d, want ~64 / ~191", left>>8, right>>8)
	}
}

func TestNormalizeAvatar_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		msg  string
	}{
		{"empty", nil, domain.MsgAvatarFormat},
		{"text", []byte("definitely not an image"), domain.MsgAvatarFormat},
		{"gif", []byte("GIF89a\x40\x00\x40\x00\x00\x00\x00;"), domain.MsgAvatarFormat},
		{"too small png", encodePNG(t, gradient(63, 200)), domain.MsgAvatarMinSide},
		{"too small webp", solidWebP(64, 10, color.NRGBA{A: 255}), domain.MsgAvatarMinSide},
		{"too wide header", pngHeaderOnly(8001, 100), domain.MsgAvatarMaxSide},
		{"too tall webp", solidWebP(100, 9000, color.NRGBA{A: 255}), domain.MsgAvatarMaxSide},
		{"truncated body", pngHeaderOnly(100, 100), domain.MsgAvatarFormat},
		{"too large file", make([]byte, domain.AvatarMaxBytes+1), domain.MsgAvatarTooBig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NormalizeAvatar(tt.data)
			assertAvatarErr(t, err, tt.msg)
		})
	}
}

// A 9000×9000 PNG must be rejected from its header: a full decode would allocate
// ~324 MB, so we bound both time and allocated memory.
func TestNormalizeAvatar_HugeRejectedBeforeDecode(t *testing.T) {
	data := pngHeaderOnly(9000, 9000)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	_, err := domain.NormalizeAvatar(data)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)

	assertAvatarErr(t, err, domain.MsgAvatarMaxSide)
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 1<<20 {
		t.Fatalf("allocated %d bytes, want < 1 MiB", alloc)
	}
	if elapsed > 100*time.Millisecond && !raceEnabled {
		t.Fatalf("took %v", elapsed)
	}
	t.Logf("rejected in %v", elapsed)
}

// A ~5 MB 4000×4000 PNG is normalized in under 2 s.
func TestNormalizeAvatar_LargePNGFast(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test: run without -race and -short")
	}
	data := largePNG(t)
	if len(data) > domain.AvatarMaxBytes || len(data) < 4<<20 {
		t.Fatalf("fixture is %d bytes, want 4–5 MiB", len(data))
	}
	start := time.Now()
	out, err := domain.NormalizeAvatar(data)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, out).Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("bounds %v", b)
	}
	t.Logf("4000x4000 PNG of %d bytes normalized in %v (output %d bytes)", len(data), elapsed, len(out))
	if elapsed > 2*time.Second {
		t.Fatalf("took %v, want < 2s", elapsed)
	}
}

// largePNG builds a 4000×4000 PNG of 4–5 MiB: a gradient (compresses well) with a
// band of noise rows (does not compress) sized to hit the target.
func largePNG(t *testing.T) []byte {
	t.Helper()
	const side = 4000
	img := gradient(side, side)
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // deterministic test noise
	// ~1.2 bytes per noisy pixel after deflate of 3 random channels at ~ 3 bytes;
	// 400 rows × 4000 px × 3 B ≈ 4.8 MB.
	for y := range 400 {
		row := img.Pix[y*img.Stride : y*img.Stride+side*4]
		for i := 0; i < len(row); i += 4 {
			v := rng.Uint32()
			row[i], row[i+1], row[i+2] = byte(v), byte(v>>8), byte(v>>16) //nolint:gosec // low bytes intended
		}
	}
	return encodePNG(t, img)
}

// JPEG has no alpha: transparent source pixels become white, not black.
func TestNormalizeAvatar_TransparentBecomesWhite(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 128, 128)) // fully transparent
	for _, data := range [][]byte{
		encodePNG(t, src),
		solidWebP(128, 128, color.NRGBA{R: 0, G: 0, B: 0, A: 0}),
	} {
		out, err := domain.NormalizeAvatar(data)
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := decodeJPEG(t, out).At(256, 256).RGBA()
		if !near(r>>8, 255) || !near(g>>8, 255) || !near(b>>8, 255) {
			t.Fatalf("transparent pixel became %d,%d,%d, want white", r>>8, g>>8, b>>8)
		}
	}
}
