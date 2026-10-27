package audioswitch

import (
	"runtime"
	"testing"

	ole "github.com/go-ole/go-ole"
	wca "github.com/moutend/go-wca"
)

// Read-only native smoke test; never changes the user's audio output.
func TestWindowsAudioEnumerationAndPolicyInterface(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		t.Fatal(err)
	}
	defer ole.CoUninitialize()
	devices, err := Devices()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range devices {
		if d.ID == "" || d.Name == "" || seen[d.ID] {
			t.Fatalf("invalid endpoint: %+v", d)
		}
		seen[d.ID] = true
		t.Logf("%s (default=%v)", d.Name, d.Default)
	}
	var policy *ole.IUnknown
	if err := wca.CoCreateInstance(ole.NewGUID("{870AF99C-171D-4F9E-AF0D-E63DF40C2BC9}"), 0, wca.CLSCTX_ALL, ole.NewGUID("{F8679F50-850A-41CF-9C72-430F290290C8}"), &policy); err != nil {
		t.Fatal(err)
	}
	policy.Release()
	if err := SetOutput("nonexistent-test-endpoint", false); err == nil {
		t.Fatal("disconnected endpoint accepted")
	}
}
