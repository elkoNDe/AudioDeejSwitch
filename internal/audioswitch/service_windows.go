package audioswitch

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/lxn/win"
)

const (
	sFalse          = 1
	rpcEChangedMode = 0x80010106
)

func initializeCOM() (func(), error) {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err == nil {
		return ole.CoUninitialize, nil
	}
	if oleErr, ok := err.(*ole.OleError); ok {
		switch uint32(oleErr.Code()) {
		case sFalse:
			// COM is already initialized on this thread; balance this successful call.
			return ole.CoUninitialize, nil
		case rpcEChangedMode:
			// The thread already uses another apartment model. Reuse it, but do not
			// uninitialize COM that this call did not initialize.
			return func() {}, nil
		}
	}
	return nil, err
}

// Start keeps COM and RegisterHotKey on their owning Windows thread. Configuration
// is independent from deej's watcher: changing shortcuts never reconnects serial.
func Start(notify func(string, string)) (stop func(), next func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	commands := make(chan struct{}, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(finished)
		cleanup, err := initializeCOM()
		if err != nil {
			notify("AudioDeejSwitch", "No se pudo iniciar el cambio de salida: "+err.Error())
			return
		}
		defer cleanup()
		user32 := syscall.NewLazyDLL("user32.dll")
		register := user32.NewProc("RegisterHotKey")
		unregister := user32.NewProc("UnregisterHotKey")
		active := map[uintptr]Binding{}
		clear := func() {
			for id := range active {
				unregister.Call(0, id)
			}
			active = map[uintptr]Binding{}
		}
		defer clear()
		var cfg Config
		last := "\x00"
		reload := func() {
			data, err := os.ReadFile(ConfigPath)
			if err != nil && !os.IsNotExist(err) {
				return
			}
			if string(data) == last {
				return
			}
			last = string(data)
			candidate, err := LoadConfig()
			if err != nil {
				notify("Atajos sin actualizar", err.Error())
				return
			}
			clear()
			cfg = candidate
			for i, b := range cfg.Bindings {
				mods, key, _ := ParseHotkey(b.Hotkey)
				id := uintptr(i + 1)
				ok, _, _ := register.Call(0, id, uintptr(mods|0x4000), uintptr(key)) // MOD_NOREPEAT
				if ok == 0 {
					notify("Atajo no disponible", b.Hotkey+" está reservado o lo utiliza otro programa.")
					continue
				}
				active[id] = b
			}
		}
		switchTo := func(target string) {
			_, err := Switch(target, false)
			if err != nil {
				notify("No se pudo cambiar la salida", err.Error())
			}
		}
		reload()
		poll := time.NewTicker(25 * time.Millisecond)
		defer poll.Stop()
		refresh := time.NewTicker(time.Second)
		defer refresh.Stop()
		for {
			select {
			case <-done:
				return
			case <-commands:
				switchTo("cycle")
			case <-refresh.C:
				reload()
			case <-poll.C:
				var msg win.MSG
				for win.PeekMessage(&msg, 0, win.WM_HOTKEY, win.WM_HOTKEY, win.PM_REMOVE) {
					if b, ok := active[uintptr(msg.WParam)]; ok {
						switchTo(b.DeviceID)
					}
				}
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done); <-finished }) }, func() {
		select {
		case commands <- struct{}{}:
		default:
		}
	}
}

func OpenSettings() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--settings")
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
