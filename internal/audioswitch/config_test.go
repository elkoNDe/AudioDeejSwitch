package audioswitch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHotkeyValidation(t *testing.T) {
	for _, text := range []string{
		"Ctrl+Alt+F8", "Shift+WIN+9", "control + a", "Alt+F24",
		"Ctrl+Oem5", "Alt+Ctrl+Oem5", "Ctrl+\\", "Shift+Oem1",
		"Ctrl+Left", "Ctrl+MediaPlayPause", "Ctrl+VK_0xDC", "Alt+Enter",
	} {
		if _, _, err := ParseHotkey(text); err != nil {
			t.Errorf("%s: %v", text, err)
		}
	}
	for _, text := range []string{
		"A", "Ctrl", "Ctrl+F12", "Ctrl++A", "Ctrl+A+B", "Ctrl+F25",
		"Ctrl+Control+A", "Oem5", "Ctrl+Oem9", "Ctrl+VK_0x01",
	} {
		if _, _, err := ParseHotkey(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
	c := Config{Bindings: []Binding{{Hotkey: "Ctrl+Alt+Oem5", DeviceID: "one"}, {Hotkey: "alt+CONTROL+oem5", DeviceID: "two"}}}
	if c.Validate() == nil {
		t.Fatal("duplicate normalized shortcut accepted")
	}
}

func TestOem5Combinations(t *testing.T) {
	for _, tc := range []struct {
		text string
		mods uint32
	}{
		{"Ctrl+Oem5", modCtrl},
		{"Alt+Ctrl+Oem5", modAlt | modCtrl},
	} {
		mods, key, err := ParseHotkey(tc.text)
		if err != nil {
			t.Errorf("%s: %v", tc.text, err)
			continue
		}
		if mods != tc.mods || key != 0xDC {
			t.Errorf("%s: got mods=%d key=%#x", tc.text, mods, key)
		}
	}
	if text := FormatHotkey(modAlt|modCtrl, 0xDC); text != "Ctrl+Alt+Oem5" {
		t.Errorf("unexpected format: %s", text)
	}
}

func TestHotkeyFormatRoundTrip(t *testing.T) {
	for _, def := range hotkeyKeyDefs {
		if def.vk == vkF12 {
			continue
		}
		text := FormatHotkey(modCtrl, def.vk)
		mods, key, err := ParseHotkey(text)
		if err != nil {
			t.Errorf("%s: %v", text, err)
			continue
		}
		if mods != modCtrl || key != def.vk {
			t.Errorf("%s: got mods=%d key=%d", text, mods, key)
		}
	}
}

func TestDeejConfigValidation(t *testing.T) {
	valid := []byte("# existing config\nslider_mapping:\n  0: master\n  1: [spotify.exe, chrome.exe]\n  2: deej.current\ncom_port: COM7\nbaud_rate: 9600\ninvert_sliders: false\nnoise_reduction: high\n")
	if err := ValidateDeejYAML(valid); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"slider_mapping: [broken",
		"com_port: COM4\nbaud_rate: 9600\n",
		"slider_mapping: {0: 12}\ncom_port: COM4\nbaud_rate: 9600\n",
		"slider_mapping: {-1: master}\ncom_port: COM4\nbaud_rate: 9600\n",
		"slider_mapping: {0: []}\ncom_port: COM4\nbaud_rate: 9600\n",
		"slider_mapping: {0: master}\ncom_port: COM4\nbaud_rate: -1\n",
	} {
		if ValidateDeejYAML([]byte(text)) == nil {
			t.Errorf("accepted invalid config %q", text)
		}
	}
}

func TestSavePreservesBackupAndRejectsExternalEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("# keep my comments\nslider_mapping: {0: master}\n")
	updated := []byte("# keep my comments\nslider_mapping: {0: mic}\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveWithBackup(path, original, updated); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil || string(backup) != string(original) {
		t.Fatalf("backup changed: %q, %v", backup, err)
	}
	if err := SaveWithBackup(path, original, []byte("stale window")); err == nil {
		t.Fatal("external edit overwritten")
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != string(updated) {
		t.Fatal("file changed after rejected save")
	}
}
