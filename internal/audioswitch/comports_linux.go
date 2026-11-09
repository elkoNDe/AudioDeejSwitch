package audioswitch

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AvailableCOMPorts returns a list of serial port device paths on Linux (e.g. /dev/ttyUSB0, /dev/ttyACM0).
func AvailableCOMPorts() ([]string, error) {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return nil, err
	}
	var ports []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") || strings.HasPrefix(name, "ttyAMA") {
			ports = append(ports, filepath.Join("/dev", name))
		}
	}
	sort.Strings(ports)
	return ports, nil
}
