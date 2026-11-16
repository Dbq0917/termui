package sixel

import (
	"image"
)

// CellSize is the pixel size of one terminal cell as reported by the
// terminal. Common terminals use 8x16.
type CellSize struct {
	Width  int
	Height int
}

// DefaultCellSize is used when the terminal does not advertise a cell size.
var DefaultCellSize = CellSize{Width: 8, Height: 16}

// Fit describes where a scaled image lands inside a cell rectangle.
type Fit struct {
	// PixelRect is the image rectangle relative to the pixel origin of the
	// requested cell area. It is centered and never exceeds the cell canvas.
	PixelRect image.Rectangle
	// CellOffsetX/CellOffsetY center the image within the requested cell area,
	// expressed in whole cells.
	CellOffsetX int
	CellOffsetY int
	// CellCols/CellRows are the whole-cell dimensions consumed by the image.
	CellCols int
	CellRows int
}

// ComputeFit scales img to fit inside cellCols x cellRows cells preserving the
// source aspect ratio. No dimension is ever stretched beyond the source ratio;
// any slack is reported as a centered offset so callers keep alignment.
func ComputeFit(img image.Image, cellCols, cellRows int, cell CellSize) Fit {
	if cell.Width <= 0 || cell.Height <= 0 {
		cell = DefaultCellSize
	}
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	maxPixelW := cellCols * cell.Width
	maxPixelH := cellRows * cell.Height

	var targetW, targetH int
	switch {
	case srcW == 0 || srcH == 0:
		targetW, targetH = 0, 0
	default:
		scale := float64(maxPixelW) / float64(srcW)
		if heightAt := float64(srcH) * scale; heightAt > float64(maxPixelH) {
			scale = float64(maxPixelH) / float64(srcH)
		}
		targetW = int(float64(srcW) * scale)
		targetH = int(float64(srcH) * scale)
		if targetW < 1 {
			targetW = 1
		}
		if targetH < 1 {
			targetH = 1
		}
	}

	usedCols := (targetW + cell.Width - 1) / cell.Width
	usedRows := (targetH + cell.Height - 1) / cell.Height
	offsetCols := (cellCols - usedCols) / 2
	offsetRows := (cellRows - usedRows) / 2

	originX := offsetCols*cell.Width + (usedCols*cell.Width-targetW)/2
	originY := offsetRows*cell.Height + (usedRows*cell.Height-targetH)/2

	return Fit{
		PixelRect:   image.Rect(originX, originY, originX+targetW, originY+targetH),
		CellOffsetX: offsetCols,
		CellOffsetY: offsetRows,
		CellCols:    usedCols,
		CellRows:    usedRows,
	}
}

// Scale resizes src to dstW x dstH. Downscaling uses area averaging so trend
// curves stay legible; upscaling uses nearest neighbor to avoid blur.
func Scale(src image.Image, dstW, dstH int) image.Image {
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	if dstW <= srcW && dstH <= srcH {
		scaleAreaAverage(src, bounds, dst, dstW, dstH)
		return dst
	}

	for y := 0; y < dstH; y++ {
		sy := y * srcH / dstH
		for x := 0; x < dstW; x++ {
			sx := x * srcW / dstW
			dst.Set(x, y, src.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return dst
}

func scaleAreaAverage(src image.Image, bounds image.Rectangle, dst *image.RGBA, dstW, dstH int) {
	srcW, srcH := bounds.Dx(), bounds.Dy()
	for dy := 0; dy < dstH; dy++ {
		sy0 := dy * srcH / dstH
		sy1 := (dy + 1) * srcH / dstH
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dstW; dx++ {
			sx0 := dx * srcW / dstW
			sx1 := (dx + 1) * srcW / dstW
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var rSum, gSum, bSum, aSum, count uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					r, g, b, a := src.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
					rSum += uint64(r >> 8)
					gSum += uint64(g >> 8)
					bSum += uint64(b >> 8)
					aSum += uint64(a >> 8)
					count++
				}
			}
			offset := dst.PixOffset(dx, dy)
			dst.Pix[offset+0] = uint8(rSum / count)
			dst.Pix[offset+1] = uint8(gSum / count)
			dst.Pix[offset+2] = uint8(bSum / count)
			dst.Pix[offset+3] = uint8(aSum / count)
		}
	}
}
