// +build windows

package sixel

// probeDeviceAttributes is unavailable on Windows; capability detection
// relies on environment variables there.
func probeDeviceAttributes() (supported bool, ok bool) {
	return false, false
}
