package domain

import (
	"bytes"
	"encoding/binary"
	"image"
)

// jpegOrientation returns the EXIF Orientation tag (0x0112, values 1–8) of a JPEG,
// or 1 when there is none or the data is not a JPEG. Only the APP1 Exif segment and
// IFD0 are read; malformed data means "no orientation".
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for p := 2; p+4 <= len(data); {
		if data[p] != 0xFF {
			return 1
		}
		marker := data[p+1]
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 || marker == 0xFF {
			p++ // standalone marker or fill byte
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image: no Exif before it
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[p+2:]))
		end := p + 2 + size
		if size < 2 || end > len(data) {
			return 1
		}
		if seg := data[p+4 : end]; marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		p = end
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd:]))
	for i := range n {
		e := ifd + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) != 0x0112 {
			continue
		}
		const typeShort = 3
		if bo.Uint16(t[e+2:]) != typeShort {
			return 1
		}
		if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}

// orient applies an EXIF orientation to a square image, so the result is what a
// viewer that honours EXIF shows. The center square of the source maps onto the
// center square of the oriented image, so cropping first and orienting after is
// equivalent and cheap (AvatarSize² pixels).
func orient(src *image.RGBA, orientation int) *image.RGBA {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	n := src.Bounds().Dx()
	dst := image.NewRGBA(image.Rect(0, 0, n, n))
	last := n - 1
	for y := range n {
		for x := range n {
			// (sx, sy) in the stored image that is displayed at (x, y).
			var sx, sy int
			switch orientation {
			case 2: // mirrored horizontally
				sx, sy = last-x, y
			case 3: // rotated 180°
				sx, sy = last-x, last-y
			case 4: // mirrored vertically
				sx, sy = x, last-y
			case 5: // transposed
				sx, sy = y, x
			case 6: // displayed rotated 90° clockwise
				sx, sy = y, last-x
			case 7: // transversed
				sx, sy = last-y, last-x
			case 8: // displayed rotated 90° counter-clockwise
				sx, sy = last-y, x
			}
			so, do := src.PixOffset(sx, sy), dst.PixOffset(x, y)
			copy(dst.Pix[do:do+4], src.Pix[so:so+4])
		}
	}
	return dst
}
