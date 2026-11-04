package deej

import (
	"reflect"
	"testing"
)

func TestParseSliderIndex(t *testing.T) {
	tests := []struct {
		value string
		want  int
		ok    bool
	}{
		{value: "0", want: 0, ok: true},
		{value: "12", want: 12, ok: true},
		{value: "-1", ok: false},
		{value: "one", ok: false},
		{value: "", ok: false},
	}

	for _, tt := range tests {
		got, ok := parseSliderIndex(tt.value)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseSliderIndex(%q) = (%d, %v), want (%d, %v)", tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSliderMapFromConfigsMergesAndFilters(t *testing.T) {
	userMapping := map[string][]string{
		"0":  {"chrome.exe", "", "spotify.exe"},
		"1":  {"deej.unmapped"},
		"x":  {"ignored.exe"},
		"-2": {"ignored.exe"},
	}

	internalMapping := map[string][]string{
		"0": {"spotify.exe", "game.exe"},
		"2": {"master"},
		"y": {"ignored.exe"},
	}

	mapping := sliderMapFromConfigs(userMapping, internalMapping)

	if targets, ok := mapping.get(0); !ok || !reflect.DeepEqual(targets, []string{"chrome.exe", "spotify.exe", "game.exe"}) {
		t.Errorf("slider 0 = %v, %v", targets, ok)
	}

	if targets, ok := mapping.get(1); !ok || !reflect.DeepEqual(targets, []string{"deej.unmapped"}) {
		t.Errorf("slider 1 = %v, %v", targets, ok)
	}

	if targets, ok := mapping.get(2); !ok || !reflect.DeepEqual(targets, []string{"master"}) {
		t.Errorf("slider 2 = %v, %v", targets, ok)
	}

	if _, ok := mapping.get(-2); ok {
		t.Error("invalid slider index was added to the map")
	}
}

func TestSliderMapString(t *testing.T) {
	mapping := newSliderMap()
	mapping.set(0, []string{"master", "mic"})
	mapping.set(1, []string{"chrome.exe"})

	if got := mapping.String(); got != "<2 sliders mapped to 3 targets>" {
		t.Errorf("String() = %q, want %q", got, "<2 sliders mapped to 3 targets>")
	}
}
