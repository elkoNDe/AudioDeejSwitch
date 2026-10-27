package audioswitch

import (
	"path/filepath"
	"testing"
)

func TestSavedNoticePreferencePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AudioDeejSwitch", "preferences.json")
	preferences, err := loadSettingsPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if preferences.HideHotkeysSaved {
		t.Fatal("the save confirmation must be visible for a new user")
	}
	preferences.HideHotkeysSaved = true
	if err := saveSettingsPreferences(path, preferences); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadSettingsPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.HideHotkeysSaved {
		t.Fatal("don't-show-again preference was lost on reload")
	}
}
