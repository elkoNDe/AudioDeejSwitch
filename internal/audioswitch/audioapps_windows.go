package audioswitch

import (
	"sort"
	"strings"
	"unsafe"

	"github.com/mitchellh/go-ps"
	wca "github.com/moutend/go-wca"
)

// RunningAudioApps enumerates all active audio render endpoint devices and
// returns a sorted list of unique executable names (lowercase) that have an
// active audio session (state = AudioSessionStateActive).
// The caller must have initialized COM (ole.CoInitializeEx) before calling.
func RunningAudioApps() []string {
	var mmdeEnumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_INPROC_SERVER, wca.IID_IMMDeviceEnumerator, &mmdeEnumerator); err != nil {
		return nil
	}
	defer mmdeEnumerator.Release()

	var mmDeviceCollection *wca.IMMDeviceCollection
	if err := mmdeEnumerator.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &mmDeviceCollection); err != nil {
		return nil
	}
	defer mmDeviceCollection.Release()

	var count uint32
	if err := mmDeviceCollection.GetCount(&count); err != nil {
		return nil
	}

	// Get all processes once to map PID -> executable name
	procs, err := ps.Processes()
	if err != nil {
		return nil
	}
	pidToExe := make(map[uint32]string, len(procs))
	for _, p := range procs {
		pidToExe[uint32(p.Pid())] = strings.ToLower(p.Executable())
	}

	seen := make(map[string]bool)
	var apps []string

	for i := uint32(0); i < count; i++ {
		var mmDevice *wca.IMMDevice
		if err := mmDeviceCollection.Item(i, &mmDevice); err != nil {
			continue
		}

		var audioSessionManager2 *wca.IAudioSessionManager2
		if err := mmDevice.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_INPROC_SERVER, nil, &audioSessionManager2); err != nil {
			mmDevice.Release()
			continue
		}

		var sessionEnumerator *wca.IAudioSessionEnumerator
		if err := audioSessionManager2.GetSessionEnumerator(&sessionEnumerator); err != nil {
			audioSessionManager2.Release()
			mmDevice.Release()
			continue
		}

		var sessionCount int
		if err := sessionEnumerator.GetCount(&sessionCount); err != nil {
			sessionEnumerator.Release()
			audioSessionManager2.Release()
			mmDevice.Release()
			continue
		}

		for j := 0; j < sessionCount; j++ {
			var sessionControl *wca.IAudioSessionControl
			if err := sessionEnumerator.GetSession(j, &sessionControl); err != nil {
				continue
			}

			// Query for IAudioSessionControl2
			dispatch, err := sessionControl.QueryInterface(wca.IID_IAudioSessionControl2)
			if err != nil {
				sessionControl.Release()
				continue
			}
			sessionControl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(dispatch))

			var state uint32
			if err := sessionControl2.GetState(&state); err == nil && state == wca.AudioSessionStateActive {
				var pid uint32
				if err := sessionControl2.GetProcessId(&pid); err == nil {
					if exe, ok := pidToExe[pid]; ok && exe != "" && !seen[exe] {
						seen[exe] = true
						apps = append(apps, exe)
					}
				}
			}
			sessionControl2.Release()
			sessionControl.Release()
		}
		sessionEnumerator.Release()
		audioSessionManager2.Release()
		mmDevice.Release()
	}

	// Sort for deterministic order
	sort.Strings(apps)
	return apps
}
