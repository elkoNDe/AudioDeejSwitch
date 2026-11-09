package audioswitch

import (
	"strings"
	"testing"
)

func TestParseSliderMapping(t *testing.T) {
	input := `
slider_mapping:
  0:
    - master
    - mic
  1:
    - system
  2:
    - deej.current
`
	entries, err := ParseSliderMapping(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Index != 0 || len(entries[0].Targets) != 2 {
		t.Errorf("entry 0: %+v", entries[0])
	}
	if entries[1].Index != 1 || len(entries[1].Targets) != 1 {
		t.Errorf("entry 1: %+v", entries[1])
	}
	if entries[2].Index != 2 || len(entries[2].Targets) != 1 {
		t.Errorf("entry 2: %+v", entries[2])
	}
}

func TestParseSliderMapping_ScalarAndList(t *testing.T) {
	input := `
slider_mapping:
  0: deej.current
  1: discord.exe   # inline comment
  2:
    - chrome.exe
    - master
invert_sliders: false
`
	entries, err := ParseSliderMapping(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Index != 0 || len(entries[0].Targets) != 1 || entries[0].Targets[0] != "deej.current" {
		t.Errorf("entry 0: %+v", entries[0])
	}
	if entries[1].Index != 1 || len(entries[1].Targets) != 1 || entries[1].Targets[0] != "discord.exe" {
		t.Errorf("entry 1: %+v", entries[1])
	}
	if entries[2].Index != 2 || len(entries[2].Targets) != 2 {
		t.Errorf("entry 2: %+v", entries[2])
	}
}

func TestParseSliderMapping_Empty(t *testing.T) {
	input := `slider_mapping:
`
	entries, err := ParseSliderMapping(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestReplaceSliderMapping(t *testing.T) {
	input := `
com_port: COM3
slider_mapping:
  0:
    - master
  1:
    - system
baud_rate: 9600
`
	entries := []SliderMappingEntry{
		{Index: 0, Targets: []string{"master", "mic"}},
		{Index: 2, Targets: []string{"deej.current"}},
	}
	out, err := ReplaceSliderMapping(input, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check that other keys preserved
	if !strings.Contains(out, "com_port: COM3") || !strings.Contains(out, "baud_rate: 9600") {
		t.Errorf("other keys not preserved:\n%s", out)
	}
	// Check new slider_mapping
	if !strings.Contains(out, "slider_mapping:") {
		t.Errorf("slider_mapping missing:\n%s", out)
	}
	if !strings.Contains(out, "0:") || !strings.Contains(out, "2:") {
		t.Errorf("indices not present:\n%s", out)
	}
	if !strings.Contains(out, "master") || !strings.Contains(out, "mic") || !strings.Contains(out, "deej.current") {
		t.Errorf("targets not present:\n%s", out)
	}
}

func TestReplaceSliderMapping_AddNew(t *testing.T) {
	input := `com_port: COM3
baud_rate: 9600
`
	entries := []SliderMappingEntry{{Index: 0, Targets: []string{"master"}}}
	out, err := ReplaceSliderMapping(input, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "slider_mapping:") {
		t.Errorf("slider_mapping not added:\n%s", out)
	}
	if !strings.Contains(out, "com_port: COM3") || !strings.Contains(out, "baud_rate: 9600") {
		t.Errorf("original keys not preserved:\n%s", out)
	}
}

func TestReplaceSliderMapping_ScalarSection(t *testing.T) {
	input := `slider_mapping:
  0: deej.current
  1: discord.exe
baud_rate: 9600
`
	entries, err := ParseSliderMapping(input)
	if err != nil {
		t.Fatal(err)
	}
	entries[1].Targets = append(entries[1].Targets, "chrome.exe")
	out, err := ReplaceSliderMapping(input, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "baud_rate: 9600") {
		t.Errorf("other keys not preserved:\n%s", out)
	}
	reparsed, err := ParseSliderMapping(out)
	if err != nil {
		t.Fatalf("output is not parseable:\n%s\n%v", out, err)
	}
	if len(reparsed) != 2 || len(reparsed[1].Targets) != 2 {
		t.Errorf("mapping not preserved:\n%s", out)
	}
}

func TestReplaceSliderMapping_EmptySectionUsesIndent(t *testing.T) {
	input := "slider_mapping:\n\nbaud_rate: 9600\n"
	entries := []SliderMappingEntry{{Index: 0, Targets: []string{"master"}}}
	out, err := ReplaceSliderMapping(input, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "slider_mapping:\n  0:\n    - master") {
		t.Errorf("expected indented mapping, got:\n%s", out)
	}
}

func TestReplaceSliderMapping_PreservesFormatting(t *testing.T) {
	input := `com_port: COM3
slider_mapping:
  0:
    - master
baud_rate: 9600
`
	entries := []SliderMappingEntry{{Index: 0, Targets: []string{"master", "mic"}}}
	out, err := ReplaceSliderMapping(input, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not introduce CRLF
	if strings.Contains(out, "\r\n") {
		t.Errorf("output contains CRLF:\n%s", out)
	}
}
