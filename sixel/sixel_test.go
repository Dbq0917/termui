package sixel

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestEncodeEmitsValidSixelEnvelope(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 7, 7))
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	seq := Encoder{MaxColors: 8}.Encode(img, 16, 24)
	if !bytes.HasPrefix(seq, []byte("\x1bPq")) {
		t.Fatalf("missing DCS introducer: %q", seq)
	}
	if !bytes.HasSuffix(seq, []byte("\x1b\\")) {
		t.Fatalf("missing ST terminator: %q", seq)
	}
	if !bytes.Contains(seq, []byte(`"1;1;7;7`)) {
		t.Fatalf("raster attributes missing dimensions: %q", seq)
	}
	if !bytes.Contains(seq, []byte("\x1b[25;17H")) {
		t.Fatalf("pixel origin not mapped from 16,24: %q", seq)
	}
	// Opaque frames must contain actual sixel glyphs (range '?'..'~').
	body := string(seq)
	if !containsRuneInRange(body, '@', '~') {
		t.Fatalf("no sixel glyphs in %q", seq)
	}
}

func TestEncodePreservesTransparency(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 6, 6))
	// Only the center 2x2 is opaque; the rest is fully transparent.
	for y := 2; y < 4; y++ {
		for x := 2; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 30, A: 255})
		}
	}

	seq := Encoder{}.Encode(img, 0, 0)
	body := string(seq)
	// Transparent gaps are represented by '?', never by painted glyphs in the
	// transparent columns. The opaque center still emits data.
	if !bytes.Contains(seq, []byte("#0")) {
		t.Fatalf("opaque region not encoded: %q", seq)
	}
	if !containsRuneInRange(body, '@', '~') {
		t.Fatalf("opaque pixels produced no glyphs: %q", seq)
	}

	// With a background, every pixel becomes opaque and no '?' gaps remain in
	// the data section.
	composited := Encoder{Background: color.RGBA{R: 1, G: 2, B: 3, A: 255}}.Encode(img, 0, 0)
	dataSection := bytes.TrimSuffix(bytes.TrimPrefix(composited, []byte("\x1bPq")), []byte("\x1b\\"))
	if bytes.Contains(dataSection, []byte("?")) {
		t.Fatalf("composited frame still contains transparent gaps: %q", composited)
	}
}

func TestEncodeCapsPaletteAt256Registers(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: uint8((x + y) * 4), A: 255})
		}
	}
	seq := Encoder{}.Encode(img, 0, 0)
	maxRegister := -1
	for i := 0; i < len(seq); i++ {
		if seq[i] != '#' {
			continue
		}
		j := i + 1
		n := 0
		for j < len(seq) && seq[j] >= '0' && seq[j] <= '9' {
			n = n*10 + int(seq[j]-'0')
			j++
		}
		if n > maxRegister {
			maxRegister = n
		}
	}
	if maxRegister >= MaxPaletteRegisters {
		t.Fatalf("palette register %d exceeds %d", maxRegister, MaxPaletteRegisters)
	}
}

func TestComputeFitPreservesAspectRatio(t *testing.T) {
	cell := CellSize{Width: 8, Height: 16}
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))

	fit := ComputeFit(img, 20, 5, cell) // canvas 160x80, height-constrained
	target := fit.PixelRect
	if target.Dx() != target.Dy() {
		t.Fatalf("square source stretched to %dx%d", target.Dx(), target.Dy())
	}
	if target.Dx() > 160 || target.Dy() > 80 {
		t.Fatalf("fit %v exceeds cell canvas", target)
	}
	if target.Min.X < 0 || target.Min.Y < 0 {
		t.Fatalf("negative fit origin: %v", target)
	}

	// Width-constrained case.
	wide := image.NewRGBA(image.Rect(0, 0, 200, 40))
	fitWide := ComputeFit(wide, 10, 10, cell) // canvas 80x160
	if fitWide.PixelRect.Dx() != 80 {
		t.Fatalf("expected width-bound fit of 80px, got %v", fitWide.PixelRect)
	}
	if ratio := float64(200) / 40; ratio != float64(fitWide.PixelRect.Dx())/float64(fitWide.PixelRect.Dy()) {
		t.Fatalf("aspect ratio changed: %v", fitWide.PixelRect)
	}
}

func TestScaleKeepsDimensionsAndContent(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.Set(0, 0, color.RGBA{R: 255, A: 255})
	src.Set(3, 1, color.RGBA{B: 255, A: 255})

	down := Scale(src, 2, 1)
	if down.Bounds().Dx() != 2 || down.Bounds().Dy() != 1 {
		t.Fatalf("downscale dimensions wrong: %v", down.Bounds())
	}
	up := Scale(src, 8, 4)
	if up.Bounds().Dx() != 8 || up.Bounds().Dy() != 4 {
		t.Fatalf("upscale dimensions wrong: %v", up.Bounds())
	}
	r, _, _, a := up.At(0, 0).RGBA()
	if r>>8 != 255 || a>>8 != 255 {
		t.Fatalf("nearest neighbor lost the red corner: %v", up.At(0, 0))
	}
}

func containsRuneInRange(s string, low, high rune) bool {
	for _, r := range s {
		if r >= low && r <= high {
			return true
		}
	}
	return false
}
