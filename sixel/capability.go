package sixel

import (
	"os"
	"strings"
	"sync"
	"time"
)

// Capability describes whether the current terminal can render SIXEL bitmaps.
type Capability struct {
	Supported bool
	// Source records how the decision was reached: "env", "da" or "default".
	Source string
}

// EnableDAProbe allows the terminal Device Attributes (DA) query to be used
// when no environment variable advertises SIXEL support. It is off by default
// because the query briefly reads from /dev/tty and must be coordinated with
// an application's own input loop.
var EnableDAProbe = false

// DATimeout bounds how long the DA probe waits for a terminal reply.
var DATimeout = 200 * time.Millisecond

var (
	capOnce  sync.Once
	capCache Capability
)

// DetectCap resolves SIXEL support once and caches the result. Override the
// decision explicitly with TERMUI_SIXEL=1/0 for locked-down deployments.
func DetectCap() Capability {
	capOnce.Do(func() {
		capCache = detectCap()
	})
	return capCache
}

// ResetCap clears the cached detection result. It is mainly useful after a
// terminal resize/reattach or in tests.
func ResetCap() {
	capOnce = sync.Once{}
	capCache = Capability{}
}

func detectCap() Capability {
	if forced, ok := envBool("TERMUI_SIXEL"); ok {
		return Capability{Supported: forced, Source: "env"}
	}

	if strings.Contains(os.Getenv("TERM_PROGRAM"), "WezTerm") ||
		strings.Contains(os.Getenv("TERM_PROGRAM"), "foot") ||
		strings.Contains(os.Getenv("TERM"), "sixel") ||
		strings.Contains(os.Getenv("TERM"), "foot") ||
		os.Getenv("KONSOLE_VERSION") != "" ||
		os.Getenv("WT_SESSION") != "" {
		return Capability{Supported: true, Source: "env"}
	}

	if os.Getenv("TERM") == "dumb" {
		return Capability{Supported: false, Source: "env"}
	}

	if EnableDAProbe {
		if supported, ok := probeDeviceAttributes(); ok {
			return Capability{Supported: supported, Source: "da"}
		}
	}

	// Fail closed: an unknown terminal keeps the byte-identical character path.
	return Capability{Supported: false, Source: "default"}
}

func envBool(key string) (bool, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}
