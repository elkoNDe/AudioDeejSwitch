package audioswitch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// UI preferences are per user and separate from the live hotkey configuration.
type settingsPreferences struct {
	HideHotkeysSaved bool `json:"hide_hotkeys_saved"`
}

func loadSettingsPreferences(path string) (settingsPreferences, error) {
	var preferences settingsPreferences
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return preferences, nil
	}
	if err != nil {
		return preferences, err
	}
	err = json.Unmarshal(data, &preferences)
	return preferences, err
}

func saveSettingsPreferences(path string, preferences settingsPreferences) error {
	data, err := json.MarshalIndent(preferences, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}
