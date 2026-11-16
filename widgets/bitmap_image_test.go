// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package widgets

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	termui "github.com/gizak/termui/v3"
)

func newTestBitmap(t *testing.T) *BitmapImage {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 6), G: uint8(y * 12), B: 128, A: 255})
		}
	}
	return NewBitmapImage(img)
}

func TestBitmapImageFallbackUsesCharacterCells(t *testing.T) {
	bmp := newTestBitmap(t)
	bmp.bitmapEnabled = func() bool { return false }
	bmp.SetRect(0, 0, 12, 8)

	buf := termui.NewBuffer(bmp.GetRect())
	bmp.Draw(buf)

	// The fallback path paints character cells inside the panel.
	innerCells := 0
	for point, cell := range buf.CellMap {
		if point.In(bmp.Inner) && cell != termui.CellClear {
			innerCells++
		}
	}
	if innerCells == 0 {
		t.Fatal("fallback mode produced no character content")
	}

	// The placement must carry the exact cell geometry so width follows cells.
	placement := bmp.GraphicPlacement()
	if placement.InnerCellRect != bmp.Inner {
		t.Fatalf("placement inner rect %v != widget inner %v", placement.InnerCellRect, bmp.Inner)
	}
	if placement.CellRect.Dx() != 12 {
		t.Fatalf("widget width not driven by cells: %v", placement.CellRect)
	}
}

func TestBitmapImageSixelModeReservesCellsOnly(t *testing.T) {
	bmp := newTestBitmap(t)
	bmp.bitmapEnabled = func() bool { return true }
	bmp.SetRect(0, 0, 12, 8)

	buf := termui.NewBuffer(bmp.GetRect())
	bmp.Draw(buf)

	// In bitmap mode the inner area stays blank: pixels come from the graphic
	// layer and never overwrite the border or neighboring color blocks.
	for point, cell := range buf.CellMap {
		if point.In(bmp.Inner) && cell != termui.CellClear {
			t.Fatalf("bitmap mode painted a character cell at %v: %+v", point, cell)
		}
	}

	borderCell, ok := buf.CellMap[image.Pt(0, 0)]
	if !ok || borderCell == termui.CellClear {
		t.Fatal("border must still be rendered through the cell buffer")
	}

	placement := bmp.GraphicPlacement()
	if !placement.InnerCellRect.In(placement.CellRect) {
		t.Fatalf("inner rect %v escapes widget rect %v", placement.InnerCellRect, placement.CellRect)
	}
}

func TestBitmapImageDoesNotEmitSixelEscapesInBuffer(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		bmp := newTestBitmap(t)
		bmp.bitmapEnabled = func() bool { return enabled }
		bmp.SetRect(0, 0, 10, 6)

		buf := termui.NewBuffer(bmp.GetRect())
		bmp.Draw(buf)

		var rendered bytes.Buffer
		for _, cell := range buf.CellMap {
			rendered.WriteRune(cell.Rune)
		}
		if bytes.Contains(rendered.Bytes(), []byte("\x1bP")) ||
			bytes.Contains(rendered.Bytes(), []byte("\x1b\\")) {
			t.Fatalf("cell buffer contains SIXEL escape sequences (enabled=%v)", enabled)
		}
	}
}

func TestBitmapImageWidthFollowsCellLayout(t *testing.T) {
	bmp := newTestBitmap(t)
	bmp.bitmapEnabled = func() bool { return true }

	for _, width := range []int{8, 20, 40} {
		bmp.SetRect(0, 0, width, 10)
		placement := bmp.GraphicPlacement()
		if placement.CellRect.Dx() != width {
			t.Fatalf("width=%d not respected: %v", width, placement.CellRect)
		}
		if placement.InnerCellRect.Dx() != width-2 {
			t.Fatalf("inner width should track bordered cells: %v", placement.InnerCellRect)
		}
	}
}

func TestSetImageBumpsContentKey(t *testing.T) {
	bmp := newTestBitmap(t)
	first := bmp.GraphicPlacement().ContentKey

	next := image.NewRGBA(image.Rect(0, 0, 4, 4))
	next.Set(0, 0, color.RGBA{R: 1, A: 255})
	bmp.SetImage(next)

	second := bmp.GraphicPlacement().ContentKey
	if second == first || second == 0 {
		t.Fatalf("SetImage did not advance content key: %d -> %d", first, second)
	}

	// Re-rendering without changing the image keeps the key stable.
	if got := bmp.GraphicPlacement().ContentKey; got != second {
		t.Fatalf("stable frame changed key unexpectedly: %d != %d", got, second)
	}
}

func TestLoadBitmapImageReadsDimensionsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.png")

	src := image.NewRGBA(image.Rect(0, 0, 47, 31)) // deliberately non-round size
	src.Set(0, 0, color.RGBA{R: 255, A: 255})
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := png.Encode(file, src); err != nil {
		t.Fatalf("encode: %v", err)
	}
	file.Close()

	bmp, err := LoadBitmapImage(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	bounds := bmp.Image.Image.Bounds()
	if bounds.Dx() != 47 || bounds.Dy() != 31 {
		t.Fatalf("dimensions must come from file, got %v", bounds)
	}
}

func TestNilImageRendersEmptyPanel(t *testing.T) {
	bmp := NewBitmapImage(nil)
	bmp.bitmapEnabled = func() bool { return true }
	bmp.SetRect(0, 0, 6, 4)

	buf := termui.NewBuffer(bmp.GetRect())
	bmp.Draw(buf) // must not panic

	placement := bmp.GraphicPlacement()
	if placement.Image != nil {
		t.Fatal("nil frame should not advertise bitmap content")
	}
}

// TestColorBlocksStayVisible ensures status color blocks rendered by other
// widgets are outside the bitmap inner rect and therefore cannot be covered.
func TestColorBlocksStayVisible(t *testing.T) {
	bmp := newTestBitmap(t)
	bmp.bitmapEnabled = func() bool { return true }
	bmp.SetRect(10, 0, 30, 8)

	statusRect := image.Rect(0, 0, 9, 1) // adjacent status strip
	placement := bmp.GraphicPlacement()
	if placement.InnerCellRect.Overlaps(statusRect) {
		t.Fatalf("bitmap inner %v overlaps status blocks %v", placement.InnerCellRect, statusRect)
	}
}

func TestFallbackOutputMatchesLegacyImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 30, 15))
	for y := 0; y < 15; y++ {
		for x := 0; x < 30; x++ {
			src.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 16), B: 90, A: 255})
		}
	}

	legacy := NewImage(src)
	legacy.SetRect(0, 0, 14, 9)
	legacyBuf := termui.NewBuffer(legacy.GetRect())
	legacy.Draw(legacyBuf)

	bitmap := NewBitmapImage(src)
	bitmap.bitmapEnabled = func() bool { return false }
	bitmap.SetRect(0, 0, 14, 9)
	bitmapBuf := termui.NewBuffer(bitmap.GetRect())
	bitmap.Draw(bitmapBuf)

	if len(bitmapBuf.CellMap) != len(legacyBuf.CellMap) {
		t.Fatalf("cell count differs: bitmap=%d legacy=%d",
			len(bitmapBuf.CellMap), len(legacyBuf.CellMap))
	}
	for point, legacyCell := range legacyBuf.CellMap {
		bitmapCell, ok := bitmapBuf.CellMap[point]
		if !ok {
			t.Fatalf("bitmap fallback missing cell at %v", point)
		}
		if bitmapCell != legacyCell {
			t.Fatalf("cell %v differs: bitmap=%+v legacy=%+v", point, bitmapCell, legacyCell)
		}
	}
}
