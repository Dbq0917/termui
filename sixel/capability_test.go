package sixel

import (
	"os"
	"testing"
)

func TestDetectCapEnvOverride(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"0", false},
		{"off", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			os.Setenv("TERMUI_SIXEL", tt.value)
			defer os.Unsetenv("TERMUI_SIXEL")
			ResetCap()

			capability := DetectCap()
			if capability.Supported != tt.want || capability.Source != "env" {
				t.Fatalf("got %+v, want supported=%v source=env", capability, tt.want)
			}
		})
	}
}

func TestDetectCapDefaultsClosed(t *testing.T) {
	for _, key := range []string{
		"TERMUI_SIXEL", "TERM_PROGRAM", "TERM", "KONSOLE_VERSION", "WT_SESSION",
	} {
		os.Unsetenv(key)
	}
	os.Setenv("TERM", "xterm")
	defer os.Unsetenv("TERM")
	ResetCap()

	capability := DetectCap()
	if capability.Supported {
		t.Fatalf("unknown terminal must default to character mode, got %+v", capability)
	}
}

func TestParseCellSize(t *testing.T) {
	size, ok := parseCellSize("10x20")
	if !ok || size.Width != 10 || size.Height != 20 {
		t.Fatalf("parse failed: %+v %v", size, ok)
	}
	if _, ok := parseCellSize("garbage"); ok {
		t.Fatal("malformed cell size must be rejected")
	}
	if got := DetectCellSize(); got.Width != 8 || got.Height != 16 {
		t.Fatalf("expected 8x16 default, got %+v", got)
	}
	os.Setenv("TERMUI_CELL_SIZE", "7x14")
	defer os.Unsetenv("TERMUI_CELL_SIZE")
	if got := DetectCellSize(); got.Width != 7 || got.Height != 14 {
		t.Fatalf("env override failed: %+v", got)
	}
}
