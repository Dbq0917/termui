package sixel

import (
	"image"
	"image/color"
)

// frame is a tightly packed, premultiplied-free RGBA representation of an
// image that the quantizer and band writer can index as y*width+x.
type frame struct {
	rgba  []uint8
	alpha []uint8
	w, h  int
}

func newFrame(img image.Image, background color.Color) *frame {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	f := &frame{
		rgba:  make([]uint8, w*h*3),
		alpha: make([]uint8, w*h),
		w:     w,
		h:     h,
	}

	var bgR, bgG, bgB uint32
	hasBackground := background != nil
	if hasBackground {
		bgR, bgG, bgB, _ = background.RGBA()
		bgR >>= 8
		bgG >>= 8
		bgB >>= 8
	}

	idx := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			a8 := uint8(a >> 8)
			f.alpha[idx] = a8

			if hasBackground {
				// Standard source-over compositing onto the background.
				af := uint32(a8)
				r = (r*af + bgR*(255-af)) / 255
				g = (g*af + bgG*(255-af)) / 255
				b = (b*af + bgB*(255-af)) / 255
				f.alpha[idx] = 255
			}

			f.rgba[idx*3] = uint8(r)
			f.rgba[idx*3+1] = uint8(g)
			f.rgba[idx*3+2] = uint8(b)
			idx++
		}
	}
	return f
}

func (f *frame) rgb(index int) (uint8, uint8, uint8) {
	return f.rgba[index*3], f.rgba[index*3+1], f.rgba[index*3+2]
}
