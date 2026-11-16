package sixel

import "sort"

type rgb struct {
	r, g, b uint8
}

type cube struct {
	pixels []int // indices into frame
}

// quantize runs a median cut over opaque pixels and returns the resulting
// palette plus a per-pixel register index. Fully transparent pixels get an
// unused index of 0 and are skipped during band writing.
func quantize(f *frame, maxColors int) ([]rgb, []uint8) {
	total := f.w * f.h
	indices := make([]uint8, total)

	opaque := make([]int, 0, total)
	for i := 0; i < total; i++ {
		if f.alpha[i] != 0 {
			opaque = append(opaque, i)
		}
	}
	if len(opaque) == 0 {
		return []rgb{{0, 0, 0}}, indices
	}

	cubes := []cube{{pixels: opaque}}
	for len(cubes) < maxColors {
		sort.Slice(cubes, func(i, j int) bool {
			return channelRange(f, cubes[i].pixels) > channelRange(f, cubes[j].pixels)
		})

		target := len(cubes) - 1
		c := cubes[target]
		if len(c.pixels) < 2 || channelRange(f, c.pixels) == 0 {
			break
		}

		left, right := splitCube(f, c)
		cubes = cubes[:target]
		cubes = append(cubes, left, right)
	}

	palette := make([]rgb, len(cubes))
	for register, c := range cubes {
		var sumR, sumG, sumB, count uint64
		for _, pixel := range c.pixels {
			r, g, b := f.rgb(pixel)
			sumR += uint64(r)
			sumG += uint64(g)
			sumB += uint64(b)
			count++
		}
		palette[register] = rgb{
			r: uint8(sumR / count),
			g: uint8(sumG / count),
			b: uint8(sumB / count),
		}
		for _, pixel := range c.pixels {
			indices[pixel] = uint8(register)
		}
	}
	return palette, indices
}

func channelRange(f *frame, pixels []int) int {
	minR, minG, minB := 255, 255, 255
	maxR, maxG, maxB := 0, 0, 0
	for _, pixel := range pixels {
		r, g, b := f.rgb(pixel)
		if int(r) < minR {
			minR = int(r)
		}
		if int(r) > maxR {
			maxR = int(r)
		}
		if int(g) < minG {
			minG = int(g)
		}
		if int(g) > maxG {
			maxG = int(g)
		}
		if int(b) < minB {
			minB = int(b)
		}
		if int(b) > maxB {
			maxB = int(b)
		}
	}
	ranges := [3]int{maxR - minR, maxG - minG, maxB - minB}
	largest := ranges[0]
	for _, span := range ranges[1:] {
		if span > largest {
			largest = span
		}
	}
	return largest
}

func splitCube(f *frame, c cube) (cube, cube) {
	channel := dominantChannel(f, c.pixels)
	sort.Slice(c.pixels, func(i, j int) bool {
		r1, g1, b1 := f.rgb(c.pixels[i])
		r2, g2, b2 := f.rgb(c.pixels[j])
		v1 := [3]uint8{r1, g1, b1}[channel]
		v2 := [3]uint8{r2, g2, b2}[channel]
		return v1 < v2
	})
	mid := len(c.pixels) / 2
	return cube{pixels: c.pixels[:mid]}, cube{pixels: c.pixels[mid:]}
}

func dominantChannel(f *frame, pixels []int) int {
	minR, minG, minB := 255, 255, 255
	maxR, maxG, maxB := 0, 0, 0
	for _, pixel := range pixels {
		r, g, b := f.rgb(pixel)
		if int(r) < minR {
			minR = int(r)
		}
		if int(r) > maxR {
			maxR = int(r)
		}
		if int(g) < minG {
			minG = int(g)
		}
		if int(g) > maxG {
			maxG = int(g)
		}
		if int(b) < minB {
			minB = int(b)
		}
		if int(b) > maxB {
			maxB = int(b)
		}
	}
	ranges := [3]int{maxR - minR, maxG - minG, maxB - minB}
	dominant := 0
	for channel := 1; channel < 3; channel++ {
		if ranges[channel] > ranges[dominant] {
			dominant = channel
		}
	}
	return dominant
}
