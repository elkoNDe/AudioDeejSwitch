package audioswitch

import (
	"strings"
	"testing"
)

func TestGetTopLevelScalar(t *testing.T) {
	input := `com_port: COM3
baud_rate: 9600
# comment
noise_reduction: low
`

	val, ok := GetTopLevelScalar(input, "com_port")
	if !ok || val != "COM3" {
		t.Errorf("expected COM3, got %q ok=%v", val, ok)
	}

	val, ok = GetTopLevelScalar(input, "baud_rate")
	if !ok || val != "9600" {
		t.Errorf("expected 9600, got %q ok=%v", val, ok)
	}

	val, ok = GetTopLevelScalar(input, "noise_reduction")
	if !ok || val != "low" {
		t.Errorf("expected low, got %q ok=%v", val, ok)
	}

	_, ok = GetTopLevelScalar(input, "nonexistent")
	if ok {
		t.Error("expected not found for nonexistent key")
	}
}

func TestSetTopLevelScalar_UpdateExisting(t *testing.T) {
	input := `com_port: COM3
baud_rate: 9600
noise_reduction: low
`
	out := SetTopLevelScalar(input, "baud_rate", "115200")
	if !strings.Contains(out, "baud_rate: 115200") {
		t.Errorf("expected baud_rate: 115200 in output, got:\n%s", out)
	}
	if strings.Count(out, "baud_rate:") != 1 {
		t.Errorf("expected exactly one baud_rate line, got:\n%s", out)
	}
	// Ensure other keys preserved
	if !strings.Contains(out, "com_port: COM3") || !strings.Contains(out, "noise_reduction: low") {
		t.Errorf("other keys not preserved:\n%s", out)
	}
}

func TestSetTopLevelScalar_AddNew(t *testing.T) {
	input := `com_port: COM3
baud_rate: 9600
`
	out := SetTopLevelScalar(input, "new_key", "new_value")
	if !strings.Contains(out, "new_key: new_value") {
		t.Errorf("expected new_key: new_value in output, got:\n%s", out)
	}
	// Ensure original keys preserved
	if !strings.Contains(out, "com_port: COM3") || !strings.Contains(out, "baud_rate: 9600") {
		t.Errorf("original keys not preserved:\n%s", out)
	}
}

func TestSetTopLevelScalar_PreservesInlineComment(t *testing.T) {
	input := `baud_rate: 9600 # this is a comment
`
	out := SetTopLevelScalar(input, "baud_rate", "115200")
	if !strings.Contains(out, "baud_rate: 115200 # this is a comment") {
		t.Errorf("inline comment not preserved, got:\n%s", out)
	}
}

func TestSetTopLevelScalar_PreservesTrailingComments(t *testing.T) {
	input := `baud_rate: 9600
# trailing comment
`
	out := SetTopLevelScalar(input, "new_key", "value")
	if !strings.Contains(out, "# trailing comment") {
		t.Errorf("trailing comment not preserved, got:\n%s", out)
	}
	// new_key should appear before trailing comment
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	foundNew := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "new_key:") {
			foundNew = true
		}
		if strings.TrimSpace(line) == "# trailing comment" && !foundNew {
			t.Errorf("new_key should appear before trailing comment, got:\n%s", out)
		}
	}
}

func TestSetTopLevelScalar_CRLF(t *testing.T) {
	input := "com_port: COM3\r\nbaud_rate: 9600\r\n"
	out := SetTopLevelScalar(input, "baud_rate", "115200")
	// Output should use LF only
	if strings.Contains(out, "\r\n") {
		t.Errorf("output should not contain CRLF, got:\n%s", out)
	}
	if !strings.Contains(out, "baud_rate: 115200") {
		t.Errorf("value not updated, got:\n%s", out)
	}
}
