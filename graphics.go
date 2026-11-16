// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package termui

import (
	"hash/fnv"
	"image"
	"image/color"
	"os"
	"sync"

	"github.com/gizak/termui/v3/sixel"
)

// GraphicPlacement is the region a bitmap widget reserves on the terminal,
// expressed in cells. All coordinates are absolute terminal cell positions.
type GraphicPlacement struct {
	// ID uniquely identifies a widget across renders so unchanged bitmaps are
	// not re-emitted. Reusing an ID with new content replaces the old bitmap.
	ID string
	// CellRect is the full widget rectangle (including border/title cells).
	CellRect image.Rectangle
	// InnerCellRect is the area where bitmap pixels may be painted.
	InnerCellRect image.Rectangle
	// Image holds the current frame. nil means nothing is painted.
	Image image.Image
	// ContentKey changes whenever the frame pixels change. Empty recomputes
	// the key from the image pixels on every render.
	ContentKey uint64
	// Background, when set, is composited behind transparent pixels so frame
	// refreshes leave no artifacts. nil preserves transparency.
	Background color.Color
}

// Graphic is implemented by Drawable widgets that own bitmap content. The
// ordinary Buffer/cell Draw path is untouched; graphics are flushed separately
// after the cell buffer so they can be diffed and updated incrementally.
type Graphic interface {
	GraphicPlacement() GraphicPlacement
}

// GraphicBackend turns placements into terminal output. The SIXEL backend is
// used by default; tests can substitute an in-memory backend.
type GraphicBackend interface {
	Available() bool
	Render(p GraphicPlacement, writer *os.File) error
}

type graphicState struct {
	placement GraphicPlacement
	signature string
}

// graphicManager keeps the last frame per widget and only emits changed
// bitmaps, which is what keeps per-second refreshes flicker-free.
type graphicManager struct {
	mu      sync.Mutex
	backend GraphicBackend
	states  map[string]graphicState
	writer  *os.File
	lastErr error
}

var graphics = newGraphicManager()

func newGraphicManager() *graphicManager {
	return &graphicManager{
		backend: &sixelBackend{encoder: sixel.Encoder{}},
		states:  make(map[string]graphicState),
	}
}

// SetGraphicBackend replaces the active graphics backend (mainly for tests).
func SetGraphicBackend(backend GraphicBackend) {
	graphics.mu.Lock()
	defer graphics.mu.Unlock()
	graphics.backend = backend
	graphics.states = make(map[string]graphicState)
}

// ResetGraphics drops cached placements. The next render repaints every
// bitmap; call it after terminal reattach or capability changes.
func ResetGraphics() {
	graphics.mu.Lock()
	defer graphics.mu.Unlock()
	graphics.states = make(map[string]graphicState)
	graphics.writer = nil
}

// GraphicError returns the last backend write error, if any. Backend failures
// never panic or break the cell rendering path.
func GraphicError() error {
	graphics.mu.Lock()
	defer graphics.mu.Unlock()
	return graphics.lastErr
}

func (m *graphicManager) openWriter() *os.File {
	if m.writer != nil {
		return m.writer
	}
	writer, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// termbox itself always opens /dev/tty on non-Windows; fall back to
		// stdout so redirected sessions still have a chance to display.
		m.writer = os.Stdout
		return m.writer
	}
	m.writer = writer
	return writer
}

func (m *graphicManager) render(items []Drawable) {
	hasGraphic := false
	for _, item := range items {
		if _, ok := item.(Graphic); ok {
			hasGraphic = true
			break
		}
	}
	if !hasGraphic {
		return
	}

	m.mu.Lock()
	backend := m.backend
	m.mu.Unlock()

	if backend == nil || !backend.Available() {
		return
	}

	writer := m.openWriter()

	seen := make(map[string]bool)
	for _, item := range items {
		graphic, ok := item.(Graphic)
		if !ok {
			continue
		}
		placement := graphic.GraphicPlacement()
		seen[placement.ID] = true

		signature := placementSignature(placement)
		m.mu.Lock()
		previous, exists := m.states[placement.ID]
		m.mu.Unlock()
		if exists && previous.signature == signature {
			continue // unchanged: no terminal traffic, no flicker
		}

		if placement.Image != nil {
			if err := backend.Render(placement, writer); err != nil {
				m.mu.Lock()
				m.lastErr = err
				m.mu.Unlock()
				continue
			}
		}

		m.mu.Lock()
		m.states[placement.ID] = graphicState{placement: placement, signature: signature}
		m.mu.Unlock()
	}

	m.mu.Lock()
	for id := range m.states {
		if !seen[id] {
			delete(m.states, id)
		}
	}
	m.mu.Unlock()
}

func placementSignature(p GraphicPlacement) string {
	key := p.ContentKey
	if key == 0 && p.Image != nil {
		key = imageContentKey(p.Image)
	}
	return joinSignature(p.ID, p.CellRect, p.InnerCellRect, key, p.Background)
}

func imageContentKey(img image.Image) uint64 {
	hasher := fnv.New64a()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			var scratch [8]byte
			for i, value := range [4]uint32{r, g, b, a} {
				scratch[i*2] = byte(value >> 16)
				scratch[i*2+1] = byte(value)
			}
			_, _ = hasher.Write(scratch[:])
		}
	}
	return hasher.Sum64()
}
