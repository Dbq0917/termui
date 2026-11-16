// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package termui

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/gizak/termui/v3/sixel"
)

func TestSixelBackendEmitsDCSSequence(t *testing.T) {
	os.Setenv("TERMUI_SIXEL", "1")
	os.Setenv("TERMUI_CELL_SIZE", "8x16")
	defer os.Unsetenv("TERMUI_SIXEL")
	defer os.Unsetenv("TERMUI_CELL_SIZE")
	sixel.ResetCap()

	backend := &sixelBackend{}
	if !backend.Available() {
		t.Fatal("capability should be forced on")
	}

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	placement := GraphicPlacement{
		ID:            "e2e",
		CellRect:      image.Rect(10, 4, 20, 9),
		InnerCellRect: image.Rect(11, 5, 19, 8), // 8 cols x 3 rows
		Image:         img,
		ContentKey:    1,
	}

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	var captured bytes.Buffer
	done := make(chan struct{})
	go func() { _, _ = captured.ReadFrom(readEnd); close(done) }()

	if err := backend.Render(placement, writeEnd); err != nil {
		t.Fatalf("render: %v", err)
	}
	_ = writeEnd.Close()
	<-done

	payload := captured.String()
	if !strings.HasPrefix(payload, "\x1bPq") || !strings.HasSuffix(payload, "\x1b[1;1H") {
		t.Fatalf("payload envelope wrong: %q", payload)
	}
	if !strings.Contains(payload, "\x1b\\") {
		t.Fatal("payload missing ST terminator")
	}
	// Inner origin is cell (11,5) at 8x16 pixels => pixel (88,80). The 16x16
	// source scales to 48x48 (height-bound, scale 3) and centers horizontally
	// in the 64px inner width, adding an 8px offset. CUP is 1-based.
	if !strings.Contains(payload, "\x1b[81;97H") {
		t.Fatalf("bitmap not centered on the inner cell origin: %q", payload)
	}
	// Raster attributes must carry the scaled, non-stretched dimensions.
	if !strings.Contains(payload, `"1;1;48;48`) {
		t.Fatalf("scaled dimensions missing from raster attributes: %q", payload)
	}
}

func TestSixelBackendUnavailableByDefault(t *testing.T) {
	for _, key := range []string{
		"TERMUI_SIXEL", "TERM_PROGRAM", "TERM", "KONSOLE_VERSION", "WT_SESSION",
	} {
		os.Unsetenv(key)
	}
	os.Setenv("TERM", "xterm-256color")
	defer os.Unsetenv("TERM")
	sixel.ResetCap()

	backend := &sixelBackend{}
	if backend.Available() {
		t.Fatal("unknown terminal must not claim SIXEL support")
	}
}

func TestFallbackRenderProducesNoSixelEscapes(t *testing.T) {
	os.Setenv("TERMUI_SIXEL", "0")
	defer os.Unsetenv("TERMUI_SIXEL")
	sixel.ResetCap()

	backend := &sixelBackend{}
	if backend.Available() {
		t.Fatal("forced-off capability still reports supported")
	}

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	var captured bytes.Buffer
	done := make(chan struct{})
	go func() { _, _ = captured.ReadFrom(readEnd); close(done) }()

	placement := GraphicPlacement{
		ID:            "fallback",
		CellRect:      image.Rect(0, 0, 5, 5),
		InnerCellRect: image.Rect(1, 1, 4, 4),
		Image:         image.NewRGBA(image.Rect(0, 0, 4, 4)),
	}
	if err := backend.Render(placement, writeEnd); err != nil {
		t.Fatalf("render: %v", err)
	}
	_ = writeEnd.Close()
	<-done

	if captured.Len() != 0 {
		t.Fatalf("disabled backend wrote data: %q", captured.String())
	}
	if bytes.Contains(captured.Bytes(), []byte("\x1bP")) {
		t.Fatal("fallback output contains a SIXEL escape sequence")
	}
}
