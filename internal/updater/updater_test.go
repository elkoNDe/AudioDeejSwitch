package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.2.0", "1.1.9", 1},
		{"2", "1.9.9", 1},
		{"1.0", "1.0.0", 0},
	}
	for _, c := range cases {
		got, err := compareVersions(c.a, c.b)
		if err != nil {
			t.Fatalf("compareVersions(%q, %q): %v", c.a, c.b, err)
		}
		if got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestReleaseDownloadURL(t *testing.T) {
	if got := releaseDownloadURL(nil); got != "" {
		t.Errorf("empty assets: got %q", got)
	}
	assets := []Asset{
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		{Name: "AudioDeejSwitch.exe", BrowserDownloadURL: "https://example.com/app.exe"},
	}
	if got := releaseDownloadURL(assets); got != "https://example.com/app.exe" {
		t.Errorf("exe asset not preferred: %q", got)
	}
	if got := releaseDownloadURL(assets[:1]); got != "https://example.com/checksums.txt" {
		t.Errorf("fallback asset: %q", got)
	}
}
