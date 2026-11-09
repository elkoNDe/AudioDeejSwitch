package audioswitch

import (
	"golang.org/x/sys/windows"
)

// EnsureSingleInstance creates a named mutex so only one instance runs per
// Windows session. The returned function releases the mutex.
func EnsureSingleInstance(appName string) (func(), error) {
	name, err := windows.UTF16PtrFromString(`Local\` + appName)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return nil, err
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(handle)
		return nil, ErrAlreadyRunning
	}
	return func() { windows.CloseHandle(handle) }, nil
}
