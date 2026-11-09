package audioswitch

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// AvailableCOMPorts returns a list of COM port names (e.g., "COM3") available on Windows.
func AvailableCOMPorts() ([]string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM`, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	var ports []string
	for _, name := range names {
		val, _, err := k.GetStringValue(name)
		if err == nil && strings.HasPrefix(strings.ToUpper(val), "COM") {
			ports = append(ports, strings.ToUpper(val))
		}
	}

	// Sort numerically: COM1, COM2, ... COM10
	sort.Slice(ports, func(i, j int) bool {
		ni, _ := strconv.Atoi(strings.TrimPrefix(ports[i], "COM"))
		nj, _ := strconv.Atoi(strings.TrimPrefix(ports[j], "COM"))
		return ni < nj
	})
	return ports, nil
}
