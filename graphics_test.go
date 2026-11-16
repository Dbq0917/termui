// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package termui

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"os"
	"sync"
	"testing"
)

// fakeGraphicBackend records payloads so tests never touch a real terminal.
type fakeGraphicBackend struct {
	mu         sync.Mutex
	available  bool
	payloads   [][]byte
	placements []GraphicPlacement
	fail       bool
}

func (f *fakeGraphicBackend) Available() bool { return f.available }

func (f *fakeGraphicBackend) Render(p GraphicPlacement, writer *os.File) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.placements = append(f.placements, p)
	payload := []byte("GRAPHIC[" + p.ID + "]")
	f.payloads = append(f.payloads, payload)
	if f.fail {
		return os.ErrInvalid
	}
	_, err := writer.Write(payload)
	return err
}

func (f *fakeGraphicBackend) renderCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.payloads)
}

type stubGraphic struct {
	Block
	id        string
	placement GraphicPlacement
}

func (s *stubGraphic) GraphicPlacement() GraphicPlacement { return s.placement }

type plainWidget struct{ Block }

func TestRenderWithoutGraphicsIsSideEffectFree(t *testing.T) {
	backend := &fakeGraphicBackend{available: true}
	SetGraphicBackend(backend)
	defer ResetGraphics()

	widget := &plainWidget{}
	widget.SetRect(0, 0, 4, 2)
	// No Graphic implementation: manager must return before touching a writer.
	graphics.render([]Drawable{widget})
	if backend.renderCount() != 0 {
		t.Fatalf("unexpected graphic output for plain widget: %d", backend.renderCount())
	}
}

func TestUnavailableBackendEmitsNothing(t *testing.T) {
	backend := &fakeGraphicBackend{available: false}
	SetGraphicBackend(backend)
	defer ResetGraphics()

	graphic := newStubGraphic("g1", image.Rect(0, 0, 10, 5), image.Rect(1, 1, 9, 4), 1)
	graphics.render([]Drawable{graphic})
	if backend.renderCount() != 0 {
		t.Fatal("disabled backend must not render bitmaps")
	}
}

func TestIncrementalRenderOnlyEmitsChangedFrames(t *testing.T) {
	backend := &fakeGraphicBackend{available: true}
	SetGraphicBackend(backend)
	ResetGraphics()

	graphics.mu.Lock()
	graphics.writer = captureWriter(t)
	graphics.mu.Unlock()

	graphic := newStubGraphic("chart", image.Rect(0, 0, 20, 8), image.Rect(1, 1, 19, 7), 11)
	renderAndCheck := func(wantRenders int) {
		t.Helper()
		graphics.render([]Drawable{graphic})
		if backend.renderCount() != wantRenders {
			t.Fatalf("render count = %d, want %d", backend.renderCount(), wantRenders)
		}
	}

	renderAndCheck(1) // first paint
	renderAndCheck(1) // identical frame: no new sixel traffic
	renderAndCheck(1) // still stable at per-second refresh cadence

	graphic.placement.ContentKey = 12 // new curve frame arrives once a minute
	renderAndCheck(2)
	renderAndCheck(2) // next second, unchanged again

	if got := bytes.Join(backend.payloads, nil); bytes.Count(got, []byte("GRAPHIC[chart]")) != 2 {
		t.Fatalf("expected exactly two bitmap frames, payloads=%q", got)
	}
}

func TestPlacementChangeRepaints(t *testing.T) {
	backend := &fakeGraphicBackend{available: true}
	SetGraphicBackend(backend)
	ResetGraphics()

	graphics.mu.Lock()
	graphics.writer = captureWriter(t)
	graphics.mu.Unlock()

	graphic := newStubGraphic("resize", image.Rect(0, 0, 20, 8), image.Rect(1, 1, 19, 7), 5)
	graphics.render([]Drawable{graphic})

	// A grid re-layout changes the cell rect: the bitmap must follow the cells.
	graphic.placement.CellRect = image.Rect(0, 0, 30, 10)
	graphic.placement.InnerCellRect = image.Rect(1, 1, 29, 9)
	graphics.render([]Drawable{graphic})

	if backend.renderCount() != 2 {
		t.Fatalf("resize must repaint, got %d renders", backend.renderCount())
	}
	last := backend.placements[len(backend.placements)-1]
	if last.InnerCellRect.Dx() != 28 {
		t.Fatalf("width did not follow cell layout: %v", last.InnerCellRect)
	}
}

func TestRemovedGraphicDropsState(t *testing.T) {
	backend := &fakeGraphicBackend{available: true}
	SetGraphicBackend(backend)
	ResetGraphics()

	graphics.mu.Lock()
	graphics.writer = captureWriter(t)
	graphics.mu.Unlock()

	first := newStubGraphic("a", image.Rect(0, 0, 4, 4), image.Rect(1, 1, 3, 3), 1)
	second := newStubGraphic("b", image.Rect(5, 0, 9, 4), image.Rect(6, 1, 8, 3), 1)
	graphics.render([]Drawable{first, second})

	graphics.render([]Drawable{first}) // second widget removed
	graphics.mu.Lock()
	_, stillThere := graphics.states["b"]
	graphics.mu.Unlock()
	if stillThere {
		t.Fatal("removed graphic state was not cleaned up")
	}
}

func TestBackendErrorDoesNotBreakRendering(t *testing.T) {
	backend := &fakeGraphicBackend{available: true, fail: true}
	SetGraphicBackend(backend)
	ResetGraphics()

	graphics.mu.Lock()
	graphics.writer = captureWriter(t)
	graphics.mu.Unlock()

	graphic := newStubGraphic("err", image.Rect(0, 0, 4, 4), image.Rect(1, 1, 3, 3), 1)
	graphics.render([]Drawable{graphic}) // must not panic
	if GraphicError() == nil {
		t.Fatal("expected GraphicError to expose the backend failure")
	}
}

func TestImageContentKeyStableForSamePixels(t *testing.T) {
	makeImage := func() image.Image {
		img := image.NewRGBA(image.Rect(0, 0, 3, 2))
		img.Set(1, 0, color.RGBA{R: 255, A: 255})
		return img
	}
	if imageContentKey(makeImage()) != imageContentKey(makeImage()) {
		t.Fatal("identical pixel data must hash identically so frames are not re-encoded")
	}
}

func newStubGraphic(id string, rect, inner image.Rectangle, key uint64) *stubGraphic {
	s := &stubGraphic{id: id}
	s.SetRect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
	s.placement = GraphicPlacement{
		ID:            id,
		CellRect:      rect,
		InnerCellRect: inner,
		Image:         image.NewRGBA(image.Rect(0, 0, 2, 2)),
		ContentKey:    key,
	}
	return s
}

// captureWriter wires the manager's *os.File output to an in-memory buffer
// using an os.Pipe, so no real terminal is needed.
func captureWriter(t *testing.T) *os.File {
	t.Helper()
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, readEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = writeEnd.Close()
		<-done
		_ = readEnd.Close()
	})
	return writeEnd
}
