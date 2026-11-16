// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package termui

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/gizak/termui/v3/sixel"
)

// sixelBackend adapts the dependency-free sixel package to GraphicBackend.
type sixelBackend struct {
	encoder sixel.Encoder
}

func (b *sixelBackend) Available() bool {
	return sixel.DetectCap().Supported
}

func (b *sixelBackend) Render(p GraphicPlacement, writer *os.File) error {
	if !b.Available() {
		return nil
	}
	cell := sixel.DetectCellSize()
	inner := p.InnerCellRect
	cellCols := inner.Dx()
	cellRows := inner.Dy()
	if cellCols <= 0 || cellRows <= 0 || p.Image == nil {
		return nil
	}

	fit := sixel.ComputeFit(p.Image, cellCols, cellRows, cell)
	target := fit.PixelRect
	scaled := sixel.Scale(p.Image, target.Dx(), target.Dy())

	encoder := sixel.Encoder{
		MaxColors:  b.encoder.MaxColors,
		Background: p.Background,
	}
	pixelX := inner.Min.X*cell.Width + target.Min.X
	pixelY := inner.Min.Y*cell.Height + target.Min.Y
	payload := encoder.Encode(scaled, pixelX, pixelY)
	if len(payload) == 0 {
		return nil
	}

	// Restore the text cursor at the end so termbox's next flush starts from a
	// known position; CUP also makes the sequence deterministic for tests.
	payload = append(payload, []byte("\x1b[1;1H")...)
	_, err := writer.Write(payload)
	if err != nil {
		return fmt.Errorf("termui: sixel write failed: %w", err)
	}
	return nil
}

func joinSignature(id string, rects ...interface{}) string {
	var builder strings.Builder
	builder.WriteString(id)
	for _, value := range rects {
		switch v := value.(type) {
		case image.Rectangle:
			fmt.Fprintf(&builder, "|r%d,%d,%d,%d", v.Min.X, v.Min.Y, v.Max.X, v.Max.Y)
		case uint64:
			fmt.Fprintf(&builder, "|k%d", v)
		case color.Color:
			r, g, b, a := v.RGBA()
			fmt.Fprintf(&builder, "|c%d,%d,%d,%d", r, g, b, a)
		default:
			fmt.Fprintf(&builder, "|%v", v)
		}
	}
	return builder.String()
}
