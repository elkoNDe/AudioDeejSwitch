package audioswitch

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	modAlt uint32 = 1 << iota
	modCtrl
	modShift
	modWin
)

const (
	vkF12        = 0x7B
	vkProcessKey = 0xE5
	vkPacket     = 0xE7
)

type hotkeyKeyDef struct {
	name    string
	vk      uint32
	aliases []string
}

var hotkeyKeyDefs = []hotkeyKeyDef{
	{name: "Backspace", vk: 0x08, aliases: []string{"Back"}},
	{name: "Tab", vk: 0x09},
	{name: "Clear", vk: 0x0C},
	{name: "Enter", vk: 0x0D, aliases: []string{"Return"}},
	{name: "Pause", vk: 0x13},
	{name: "CapsLock", vk: 0x14, aliases: []string{"Capital"}},
	{name: "Escape", vk: 0x1B, aliases: []string{"Esc"}},
	{name: "Space", vk: 0x20},
	{name: "PageUp", vk: 0x21, aliases: []string{"Prior", "PgUp"}},
	{name: "PageDown", vk: 0x22, aliases: []string{"Next", "PgDn"}},
	{name: "End", vk: 0x23},
	{name: "Home", vk: 0x24},
	{name: "Left", vk: 0x25},
	{name: "Up", vk: 0x26},
	{name: "Right", vk: 0x27},
	{name: "Down", vk: 0x28},
	{name: "Select", vk: 0x29},
	{name: "Print", vk: 0x2A},
	{name: "Execute", vk: 0x2B},
	{name: "PrintScreen", vk: 0x2C, aliases: []string{"Snapshot"}},
	{name: "Insert", vk: 0x2D, aliases: []string{"Ins"}},
	{name: "Delete", vk: 0x2E, aliases: []string{"Del"}},
	{name: "Help", vk: 0x2F},
	{name: "Apps", vk: 0x5D, aliases: []string{"ContextMenu"}},
	{name: "Sleep", vk: 0x5F},
	{name: "Multiply", vk: 0x6A, aliases: []string{"NumpadMultiply"}},
	{name: "Add", vk: 0x6B, aliases: []string{"NumpadAdd"}},
	{name: "Separator", vk: 0x6C, aliases: []string{"NumpadSeparator"}},
	{name: "Subtract", vk: 0x6D, aliases: []string{"NumpadSubtract"}},
	{name: "Decimal", vk: 0x6E, aliases: []string{"NumpadDecimal"}},
	{name: "Divide", vk: 0x6F, aliases: []string{"NumpadDivide"}},
	{name: "NumLock", vk: 0x90},
	{name: "ScrollLock", vk: 0x91, aliases: []string{"Scroll"}},
	{name: "BrowserBack", vk: 0xA6},
	{name: "BrowserForward", vk: 0xA7},
	{name: "BrowserRefresh", vk: 0xA8},
	{name: "BrowserStop", vk: 0xA9},
	{name: "BrowserSearch", vk: 0xAA},
	{name: "BrowserFavorites", vk: 0xAB},
	{name: "BrowserHome", vk: 0xAC},
	{name: "VolumeMute", vk: 0xAD},
	{name: "VolumeDown", vk: 0xAE},
	{name: "VolumeUp", vk: 0xAF},
	{name: "MediaNextTrack", vk: 0xB0},
	{name: "MediaPrevTrack", vk: 0xB1},
	{name: "MediaStop", vk: 0xB2},
	{name: "MediaPlayPause", vk: 0xB3},
	{name: "LaunchMail", vk: 0xB4},
	{name: "LaunchMediaSelect", vk: 0xB5},
	{name: "LaunchApp1", vk: 0xB6},
	{name: "LaunchApp2", vk: 0xB7},
	{name: "Oem1", vk: 0xBA, aliases: []string{";", "Semicolon"}},
	{name: "OemPlus", vk: 0xBB, aliases: []string{"=", "Equal"}},
	{name: "OemComma", vk: 0xBC, aliases: []string{",", "Comma"}},
	{name: "OemMinus", vk: 0xBD, aliases: []string{"-", "Minus"}},
	{name: "OemPeriod", vk: 0xBE, aliases: []string{".", "Period"}},
	{name: "Oem2", vk: 0xBF, aliases: []string{"/", "Slash"}},
	{name: "Oem3", vk: 0xC0, aliases: []string{"`", "Backtick"}},
	{name: "Oem4", vk: 0xDB, aliases: []string{"[", "LeftBracket"}},
	{name: "Oem5", vk: 0xDC, aliases: []string{"\\", "Backslash"}},
	{name: "Oem6", vk: 0xDD, aliases: []string{"]", "RightBracket"}},
	{name: "Oem7", vk: 0xDE, aliases: []string{"'", "Quote"}},
	{name: "Oem8", vk: 0xDF},
	{name: "Oem102", vk: 0xE2},
}

var (
	hotkeyNameToVK = map[string]uint32{}
	hotkeyVKToName = map[uint32]string{}
)

func init() {
	for c := byte('A'); c <= 'Z'; c++ {
		hotkeyKeyDefs = append(hotkeyKeyDefs, hotkeyKeyDef{name: string(c), vk: uint32(c)})
	}
	for c := byte('0'); c <= '9'; c++ {
		hotkeyKeyDefs = append(hotkeyKeyDefs, hotkeyKeyDef{name: string(c), vk: uint32(c)})
	}
	for i := 1; i <= 24; i++ {
		hotkeyKeyDefs = append(hotkeyKeyDefs, hotkeyKeyDef{name: fmt.Sprintf("F%d", i), vk: uint32(0x70 + i - 1)})
	}
	for i := 0; i <= 9; i++ {
		hotkeyKeyDefs = append(hotkeyKeyDefs, hotkeyKeyDef{name: fmt.Sprintf("Numpad%d", i), vk: uint32(0x60 + i)})
	}

	for _, def := range hotkeyKeyDefs {
		hotkeyVKToName[def.vk] = def.name
		hotkeyNameToVK[strings.ToUpper(def.name)] = def.vk
		for _, alias := range def.aliases {
			hotkeyNameToVK[strings.ToUpper(alias)] = def.vk
		}
	}
}

// ParseHotkey returns the Win32 modifier mask and virtual key. Keys are
// physical keyboard keys, not characters: Shift+Oem1 may type ':' depending on
// the keyboard layout. A modifier is required so ordinary typing is not captured.
func ParseHotkey(s string) (uint32, uint32, error) {
	var mods, key uint32
	for _, raw := range strings.Split(strings.TrimSpace(s), "+") {
		part := strings.ToUpper(strings.TrimSpace(raw))
		if part == "" {
			return 0, 0, fmt.Errorf("atajo inválido: %s", s)
		}

		var mod uint32
		switch part {
		case "ALT":
			mod = modAlt
		case "CTRL", "CONTROL":
			mod = modCtrl
		case "SHIFT":
			mod = modShift
		case "WIN", "WINDOWS":
			mod = modWin
		default:
			if key != 0 {
				return 0, 0, fmt.Errorf("atajo inválido: %s", s)
			}
			vk, ok := parseHotkeyKey(part)
			if !ok {
				return 0, 0, fmt.Errorf("tecla no admitida en %q (usa nombres como Oem5, F8, Left o VK_0xDC)", s)
			}
			key = vk
			continue
		}

		if mods&mod != 0 {
			return 0, 0, fmt.Errorf("modificador repetido: %s", s)
		}
		mods |= mod
	}

	if err := ValidateHotkey(mods, key); err != nil {
		return 0, 0, err
	}
	return mods, key, nil
}

func parseHotkeyKey(part string) (uint32, bool) {
	if vk, ok := hotkeyNameToVK[part]; ok {
		return vk, true
	}
	if !strings.HasPrefix(part, "VK_") {
		return 0, false
	}

	number := strings.TrimPrefix(part, "VK_")
	base := 10
	if strings.HasPrefix(number, "0X") {
		number = strings.TrimPrefix(number, "0X")
		base = 16
	}
	vk, err := strconv.ParseUint(number, base, 32)
	if err != nil {
		return 0, false
	}
	return uint32(vk), true
}

func ValidateHotkey(mods, key uint32) error {
	if mods == 0 {
		return fmt.Errorf("usa un atajo con Ctrl, Alt, Shift o Win")
	}
	if mods&^uint32(modAlt|modCtrl|modShift|modWin) != 0 {
		return fmt.Errorf("modificadores no admitidos")
	}
	if key == 0 || key > 0xFF {
		return fmt.Errorf("tecla no admitida")
	}
	if isModifierKey(key) {
		return fmt.Errorf("la tecla debe ser distinta de Ctrl, Alt, Shift o Win")
	}
	switch key {
	case 0x01, 0x02, 0x04, 0x05, 0x06:
		return fmt.Errorf("los botones del ratón no se pueden usar como atajo")
	case vkF12:
		return fmt.Errorf("F12 está reservada por Windows")
	case vkProcessKey, vkPacket:
		return fmt.Errorf("esa tecla no se puede usar como atajo")
	}
	return nil
}

func FormatHotkey(mods, key uint32) string {
	parts := make([]string, 0, 5)
	if mods&modCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&modShift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&modWin != 0 {
		parts = append(parts, "Win")
	}

	name, ok := hotkeyVKToName[key]
	if !ok {
		name = fmt.Sprintf("VK_0x%02X", key)
	}
	parts = append(parts, name)
	return strings.Join(parts, "+")
}

func isModifierKey(key uint32) bool {
	switch key {
	case 0x10, 0x11, 0x12, 0x5B, 0x5C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5:
		return true
	}
	return false
}
