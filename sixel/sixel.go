// Package sixel provides a dependency-free SIXEL encoder, aspect-correct
// image scaling and terminal capability detection for bitmap rendering on
// SIXEL-capable terminals.
package sixel

import (
	"bytes"
	"image"
	"image/color"
	"strconv"
)

// MaxPaletteRegisters is the maximum number of color registers SIXEL allows.
const MaxPaletteRegisters = 256

// Encoder turns an image.Image into a SIXEL DCS escape sequence.
//
// The zero value is ready to use. When Background is nil, fully transparent
// pixels are skipped so whatever is already on the terminal (including cell
// based color blocks) stays visible instead of being painted black. When a
// background color is supplied, the source is alpha-composited onto it first,
// which produces a self-contained opaque frame that can be refreshed without
// leaving artifacts from the previous frame.
type Encoder struct {
	// MaxColors bounds the number of SIXEL color registers used (1..256).
	// Zero defaults to MaxPaletteRegisters.
	MaxColors int
	// Background, when non-nil, is composited under semi/transparent pixels.
	Background color.Color
}

// Encode returns the full DCS sequence that draws img at pixel offset
// (pixelX, pixelY) on the terminal. The image must already be scaled to the
// desired pixel size; see Fit and Scale.
func (e Encoder) Encode(img image.Image, pixelX, pixelY int) []byte {
	maxColors := e.MaxColors
	if maxColors <= 0 || maxColors > MaxPaletteRegisters {
		maxColors = MaxPaletteRegisters
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil
	}

	frame := newFrame(img, e.Background)
	palette, indices := quantize(frame, maxColors)

	var buf bytes.Buffer
	// DCS introducer, 7-bit controls mode.
	buf.WriteString("\x1bPq")
	// Raster attributes: pan/pad 1, explicit image dimensions.
	buf.WriteString(`"1;1;`)
	writeDecimal(&buf, width)
	buf.WriteByte(';')
	writeDecimal(&buf, height)

	// Color register definitions using the RGB percentage format.
	for register, c := range palette {
		buf.WriteByte('#')
		writeDecimal(&buf, register)
		buf.WriteString(";2;")
		writeDecimal(&buf, percent(c.r))
		buf.WriteByte(';')
		writeDecimal(&buf, percent(c.g))
		buf.WriteByte(';')
		writeDecimal(&buf, percent(c.b))
	}

	// Move the active cursor to the requested pixel origin.
	writePosition(&buf, pixelX, pixelY)

	for bandTop := 0; bandTop < height; bandTop += 6 {
		bandHeight := 6
		if bandTop+bandHeight > height {
			bandHeight = height - bandTop
		}
		for register := range palette {
			writeColorBand(&buf, frame, indices, width, bandTop, bandHeight, register)
		}
		if bandTop+6 < height {
			buf.WriteByte('-') // move to the next sixel band
		}
	}

	// ST terminates the DCS string.
	buf.WriteString("\x1b\\")
	return buf.Bytes()
}

// writeColorBand emits one color register's rasters for a single band.
// Transparent pixels never emit data, and '$' returns the cursor to the band
// origin so the next register starts at the same column.
func writeColorBand(buf *bytes.Buffer, frame *frame, indices []uint8, width, bandTop, bandHeight, register int) {
	wrote := false
	for x := 0; x < width; {
		bits := byte(0)
		opaque := false
		for row := 0; row < bandHeight; row++ {
			idx := (bandTop+row)*width + x
			if frame.alpha[idx] == 0 {
				continue
			}
			opaque = true
			if int(indices[idx]) == register {
				bits |= 1 << uint(row)
			}
		}

		emitted := false
		if opaque && bits != 0 {
			emitted = true
		}
		if !wrote {
			buf.WriteByte('#')
			writeDecimal(buf, register)
			wrote = true
		}

		// RLE identical columns, including transparent gaps represented by
		// '?', which advances the cursor without painting any raster.
		run := 1
		for x+run < width {
			nextBits := byte(0)
			nextOpaque := false
			for row := 0; row < bandHeight; row++ {
				idx := (bandTop+row)*width + x + run
				if frame.alpha[idx] == 0 {
					continue
				}
				nextOpaque = true
				if int(indices[idx]) == register {
					nextBits |= 1 << uint(row)
				}
			}
			nextEmitted := nextOpaque && nextBits != 0
			if nextEmitted != emitted || (emitted && nextBits != bits) {
				break
			}
			run++
		}
		if emitted {
			writeRun(buf, sixelChar(bits), run)
		} else {
			writeRun(buf, '?', run)
		}
		x += run
	}

	if wrote {
		buf.WriteByte('$')
	}
}

func writeRun(buf *bytes.Buffer, glyph byte, count int) {
	if count > 3 {
		buf.WriteByte('!')
		writeDecimal(buf, count)
	} else {
		for i := 0; i < count; i++ {
			buf.WriteByte(glyph)
		}
		return
	}
	buf.WriteByte(glyph)
}

func writeDecimal(buf *bytes.Buffer, value int) {
	buf.WriteString(strconv.Itoa(value))
}

func writePosition(buf *bytes.Buffer, x, y int) {
	buf.WriteString("\x1b[")
	writeDecimal(buf, y+1)
	buf.WriteByte(';')
	writeDecimal(buf, x+1)
	buf.WriteByte('H')
}

func sixelChar(bits byte) byte {
	return byte('?') + bits
}

func percent(channel uint8) int {
	return int(channel) * 100 / 255
}
