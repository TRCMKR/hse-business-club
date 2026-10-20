package domain

import (
	"bytes"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG for image.DecodeConfig/Decode

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP for image.DecodeConfig/Decode

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// Profile photo limits and output parameters (SPEC §2.4).
const (
	AvatarMaxBytes  = 5 << 20 // 5 MiB source file
	AvatarMinSide   = 64
	AvatarMaxSide   = 8000
	AvatarSize      = 512 // output is AvatarSize×AvatarSize
	AvatarQuality   = 85
	AvatarMediaType = "image/jpeg"
)

// User-facing photo messages (SPEC §2.3).
const (
	MsgAvatarFormat  = "Фото: поддерживаются JPEG, PNG и WebP"
	MsgAvatarTooBig  = "Фото: не больше 5 МБ"
	MsgAvatarMaxSide = "Фото: не больше 8000×8000"
	MsgAvatarMinSide = "Фото: минимум 64×64"
)

var avatarFormats = map[string]bool{"jpeg": true, "png": true, "webp": true}

// NormalizeAvatar validates a source photo and converts it into a 512×512 JPEG (q85):
// the centered square is cut out and resized with Catmull-Rom over a white background
// (transparency becomes white). Size, format and
// dimensions are checked from the image header before the image is decoded, which
// protects from decompression bombs. The EXIF orientation of a JPEG (photos from a
// phone camera) is applied, so the result looks like the preview in a browser;
// other metadata of the source is not carried over. Errors are apperr.Validation("avatar", …).
func NormalizeAvatar(data []byte) ([]byte, error) {
	if len(data) > AvatarMaxBytes {
		return nil, apperr.Validation(FieldAvatar, MsgAvatarTooBig)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !avatarFormats[format] {
		return nil, apperr.Validation(FieldAvatar, MsgAvatarFormat)
	}
	if cfg.Width > AvatarMaxSide || cfg.Height > AvatarMaxSide {
		return nil, apperr.Validation(FieldAvatar, MsgAvatarMaxSide)
	}
	if cfg.Width < AvatarMinSide || cfg.Height < AvatarMinSide {
		return nil, apperr.Validation(FieldAvatar, MsgAvatarMinSide)
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.Validation(FieldAvatar, MsgAvatarFormat)
	}

	// JPEG has no alpha: transparent areas are composited over white, not black.
	dst := image.NewRGBA(image.Rect(0, 0, AvatarSize, AvatarSize))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, centerSquare(src.Bounds()), draw.Over, nil)
	if format == "jpeg" {
		dst = orient(dst, jpegOrientation(data))
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: AvatarQuality}); err != nil {
		return nil, apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", err)
	}
	return buf.Bytes(), nil
}

// centerSquare returns the largest square centered in r.
func centerSquare(r image.Rectangle) image.Rectangle {
	w, h := r.Dx(), r.Dy()
	side := min(w, h)
	x0 := r.Min.X + (w-side)/2
	y0 := r.Min.Y + (h-side)/2
	return image.Rect(x0, y0, x0+side, y0+side)
}
