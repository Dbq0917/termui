// +build !windows

package sixel

import (
	"os"
	"strings"
	"time"
)

// probeDeviceAttributes sends the Primary Device Attributes request (CSI c)
// and looks for SIXEL support (attribute 4) in the reply. The boolean result
// is only trustworthy when ok is true.
func probeDeviceAttributes() (supported bool, ok bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, false
	}
	defer tty.Close()

	if _, err := tty.WriteString("\x1b[c"); err != nil {
		return false, false
	}

	deadline := time.Now().Add(DATimeout)
	buf := make([]byte, 0, 128)
	chunk := make([]byte, 64)
	for {
		wait := deadline.Sub(time.Now())
		if wait <= 0 {
			return false, false
		}
		if err := tty.SetReadDeadline(time.Now().Add(wait)); err != nil {
			return false, false
		}
		n, err := tty.Read(chunk)
		if err != nil || n == 0 {
			return false, false
		}
		buf = append(buf, chunk[:n]...)
		if reply := string(buf); strings.Contains(reply, "c") {
			// Replies look like ESC [ ? 64; 1; 2; 4 c. Attribute 4 = SIXEL.
			fields := strings.FieldsFunc(reply, func(r rune) bool {
				return r == ';' || r == ' ' || r == '?' || r == '[' || r == 'c' || r == 0x1b
			})
			for _, field := range fields {
				if field == "4" {
					return true, true
				}
			}
			return false, true
		}
	}
}
