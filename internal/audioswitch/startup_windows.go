package audioswitch

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// StartupEnabled returns true if the app is registered in HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
func StartupEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	_, _, err = k.GetStringValue("AudioDeejSwitch")
	if err == registry.ErrNotExist {
		return false, nil
	}
	return err == nil, err
}

// SetStartupEnabled adds or removes the app from HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
func SetStartupEnabled(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enabled {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		return k.SetStringValue("AudioDeejSwitch", `"`+execPath+`"`)
	}
	if err := k.DeleteValue("AudioDeejSwitch"); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
