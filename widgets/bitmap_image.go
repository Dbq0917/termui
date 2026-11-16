// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package widgets

import (
	"fmt"
	stdimage "image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	termui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/sixel"
)

// BitmapImage renders an image.Image as a real bitmap on SIXEL-capable
// terminals and transparently falls back to the character-cell Image widget
// otherwise. It is a normal Drawable: layout/width calculation is driven
// entirely by its cell rectangle, so it participates in grids exactly like
// every other widget.
type BitmapImage struct {
	*Image

	// Background composites under transparent pixels when non-nil. Leave it
	// nil to preserve transparency (nothing underneath is painted black).
	Background color.Color

	// bitmapEnabled caches terminal capability so Draw stays allocation-light.
	bitmapEnabled func() bool

	// contentKey bumps whenever a new frame is assigned; zero means "recompute
	// from pixels". It lets unchanged frames skip re-encoding entirely.
	contentKey uint64
	frameSeq   uint64
}

// NewBitmapImage constructs a bitmap widget. A nil image is allowed and
// renders as an empty panel until SetImage supplies a frame.
func NewBitmapImage(frame stdimage.Image) *BitmapImage {
	bitmap := &BitmapImage{
		bitmapEnabled: defaultBitmapEnabled,
	}
	bitmap.Image = NewImage(frame)
	return bitmap
}

// LoadBitmapImage decodes an image file. Pixel dimensions come from the
// decoded header, never hard-coded. PNG, JPEG and GIF are registered by the
// package; additional formats can be registered by the application.
func LoadBitmapImage(path string) (*BitmapImage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("bitmap image: open %s: %w", path, err)
	}
	defer file.Close()

	img, _, err := stdimage.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("bitmap image: decode %s: %w", path, err)
	}
	return NewBitmapImage(img), nil
}

// SetImage replaces the frame and bumps its content key. The bitmap is only
// re-encoded on the next Render, and only when the pixels actually changed.
func (b *BitmapImage) SetImage(frame stdimage.Image) {
	b.Lock()
	defer b.Unlock()
	b.Image.Image = frame
	b.frameSeq++
	b.contentKey = b.frameSeq
}

// Touch forces the current frame to be re-encoded after in-place pixel
// mutation, since SetImage cannot observe mutations done through the same
// underlying image.
func (b *BitmapImage) Touch() {
	b.Lock()
	defer b.Unlock()
	b.frameSeq++
	b.contentKey = b.frameSeq
}

// Draw keeps the Buffer contract intact. The border/title always go through
// the normal cell path so adjacent color blocks and layout are unaffected; on
// capable terminals the inner cells stay blank placeholders while the bitmap
// layer paints the pixels, otherwise the legacy character renderer is used.
func (b *BitmapImage) Draw(buf *termui.Buffer) {
	b.Block.Draw(buf)

	if b.Image.Image == nil {
		return
	}

	if b.bitmapEnabled != nil && b.bitmapEnabled() {
		b.reserveInnerCells(buf)
		return
	}

	b.drawCharacterFallback(buf)
}

// GraphicPlacement implements termui.Graphic. The rectangles are absolute cell
// coordinates, which keeps bitmap width aligned to the grid even after
// re-layout, and guarantees the bitmap never covers the border or neighboring
// status color blocks.
func (b *BitmapImage) GraphicPlacement() termui.GraphicPlacement {
	b.Lock()
	defer b.Unlock()
	return termui.GraphicPlacement{
		ID:            fmt.Sprintf("bitmap-%p", b),
		CellRect:      b.Rectangle,
		InnerCellRect: b.Inner,
		Image:         b.Image.Image,
		ContentKey:    b.contentKey,
		Background:    b.Background,
	}
}

// reserveInnerCells fills the inner area with clear cells so the cell buffer
// stays well-defined and the grid keeps its expected width.
func (b *BitmapImage) reserveInnerCells(buf *termui.Buffer) {
	buf.Fill(termui.CellClear, b.Inner)
}

// drawCharacterFallback delegates to the existing character-cell renderer, so
// unsupported terminals get exactly the legacy widgets.Image behavior.
func (b *BitmapImage) drawCharacterFallback(buf *termui.Buffer) {
	b.Image.Draw(buf)
}

func defaultBitmapEnabled() bool {
	return sixel.DetectCap().Supported
}
