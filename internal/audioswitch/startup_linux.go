package audioswitch

import (
	"os"
	"path/filepath"
)

// StartupEnabled returns true if the app is configured to start on login.
func StartupEnabled() (bool, error) {
	desktop := os.Getenv("XDG_CONFIG_HOME")
	if desktop == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, err
		}
		desktop = filepath.Join(home, ".config")
	}
	autostartDir := filepath.Join(desktop, "autostart")
	desktopFile := filepath.Join(autostartDir, "AudioDeejSwitch.desktop")
	_, err := os.Stat(desktopFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// SetStartupEnabled enables or disables autostart on Linux by creating/removing
// a .desktop file in ~/.config/autostart/.
func SetStartupEnabled(enabled bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	autostartDir := filepath.Join(home, ".config", "autostart")
	desktopFile := filepath.Join(autostartDir, "AudioDeejSwitch.desktop")

	if enabled {
		if err := os.MkdirAll(autostartDir, 0755); err != nil {
			return err
		}
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		content := `[Desktop Entry]
Type=Application
Name=AudioDeejSwitch
Exec=` + execPath + `
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
`
		return os.WriteFile(desktopFile, []byte(content), 0644)
	}
	return os.Remove(desktopFile)
}
