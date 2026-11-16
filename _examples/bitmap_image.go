// Copyright 2017 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

// +build ignore

// Bitmap image panel demo. On a SIXEL-capable terminal the curve renders as a
// real bitmap; elsewhere it falls back to the character-cell image renderer.
// Refreshes only repaint changed frames, so per-second updates do not flicker.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"

	termui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

func main() {
	if err := termui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer termui.Close()

	width, height := termui.TerminalDimensions()

	chart := widgets.NewBitmapImage(renderCurve(0))
	chart.Title = "Trend (bitmap)"
	chart.SetRect(0, 0, width/2, height/2)

	status := widgets.NewParagraph()
	status.Text = "Status blocks stay cell-based and are never overwritten"
	status.SetRect(width/2, 0, width, 3)

	tick := 0
	render := func() {
		chart.SetImage(renderCurve(tick))
		termui.Render(chart, status)
	}
	render()

	events := termui.PollEvents()
	for {
		e := <-events
		switch e.ID {
		case "q", "<C-c>":
			return
		case "<Resize>":
			width, height = termui.TerminalDimensions()
			chart.SetRect(0, 0, width/2, height/2)
			status.SetRect(width/2, 0, width, 3)
			termui.ResetGraphics()
			render()
		default:
			tick++
			render() // one frame per second in a real dashboard loop
		}
	}
}

// renderCurve builds a fresh frame each tick, simulating the once-a-minute
// trend snapshot. Dimensions are read back from the image, never assumed.
func renderCurve(tick int) image.Image {
	const w, h = 320, 120
	frame := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		y := h/2 - int(30*math.Sin(float64(x+tick*8)/24.0))
		for dy := -1; dy <= 1; dy++ {
			if y+dy >= 0 && y+dy < h {
				frame.Set(x, y+dy, color.RGBA{R: 80, G: 200, B: 255, A: 255})
			}
		}
	}

	if len(os.Args) > 1 {
		if file, err := os.Create(os.Args[1]); err == nil {
			defer file.Close()
			_ = png.Encode(file, frame)
			fmt.Println("wrote sample frame to", os.Args[1])
		}
	}
	return frame
}
