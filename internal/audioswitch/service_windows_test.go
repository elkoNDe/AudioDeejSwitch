package audioswitch

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestGlobalHotkeyReloadAndCleanup(t *testing.T) {
	// Work in an isolated directory and never send the hotkey: no output changes.
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	u := syscall.NewLazyDLL("user32.dll")
	register := u.NewProc("RegisterHotKey")
	unregister := u.NewProc("UnregisterHotKey")
	const id = 200
	// True means our probe acquired it, false means the service owns it.
	available := func(key uintptr) bool {
		ok, _, _ := register.Call(0, id, 2|1|4|0x4000, key)
		if ok != 0 {
			unregister.Call(0, id)
		}
		return ok != 0
	}
	if !available(0x86) || !available(0x85) {
		t.Skip("test shortcuts already occupied")
	} // F23, F22
	write := func(key string) {
		b := []byte("communications: false\nbindings:\n  - hotkey: Ctrl+Alt+Shift+" + key + "\n    device_id: cycle\n    name: test\n")
		if err := os.WriteFile(filepath.Join(dir, ConfigPath), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	waitOwned := func(key uintptr) {
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if !available(key) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("service did not register shortcut")
	}
	write("F23")
	notifications := make(chan string, 10)
	stop, _ := Start(func(title, message string) { notifications <- title + ": " + message })
	defer stop()
	waitOwned(0x86)
	write("F22")
	waitOwned(0x85)
	if !available(0x86) {
		t.Fatal("old shortcut not released on reload")
	}
	stop()
	if !available(0x85) {
		t.Fatal("shortcut not released on shutdown")
	}
	select {
	case message := <-notifications:
		t.Fatal(message)
	default:
	}
}
