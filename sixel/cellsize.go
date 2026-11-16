package sixel

import (
	"os"
	"strconv"
	"strings"
)

// DetectCellSize resolves the pixel size of one terminal cell. Explicit
// TERMUI_CELL_SIZE="widthxheight" wins, otherwise the conservative 8x16
// default is used; callers re-detect on resize so scaling follows the
// terminal resolution.
func DetectCellSize() CellSize {
	if raw := os.Getenv("TERMUI_CELL_SIZE"); raw != "" {
		if size, ok := parseCellSize(raw); ok {
			return size
		}
	}
	return DefaultCellSize
}

func parseCellSize(raw string) (CellSize, bool) {
	parts := strings.Split(strings.ToLower(raw), "x")
	if len(parts) != 2 {
		return CellSize{}, false
	}
	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return CellSize{}, false
	}
	return CellSize{Width: width, Height: height}, true
}
