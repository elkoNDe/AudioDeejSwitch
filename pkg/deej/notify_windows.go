package deej

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/lxn/win"
)

// The main tray is implemented with walk's NotifyIcon, which uses the
// Walk_MainWindow_Class window and notification area ID 0.
const walkMainWindowClass = `\o/ Walk_MainWindow_Class \o/`

const trayNotificationID = 0

var (
	enumWindows        = syscall.NewLazyDLL("user32.dll").NewProc("EnumWindows")
	traySearchMu       sync.Mutex
	traySearchPID      uint32
	traySearchResult   win.HWND
	enumWindowsProcess = syscall.NewCallback(func(hwnd win.HWND, _ uintptr) uintptr {
		var pid uint32
		win.GetWindowThreadProcessId(hwnd, &pid)
		if pid != traySearchPID {
			return 1
		}
		var class [64]uint16
		n, err := win.GetClassName(hwnd, &class[0], len(class))
		if err != nil || n <= 0 {
			return 1
		}
		if syscall.UTF16ToString(class[:n]) == walkMainWindowClass {
			traySearchResult = hwnd
			return 0
		}
		return 1
	})
)

// The Windows implementation deliberately has no dependency on beeep or toast:
// their legacy Windows backends generate and execute PowerShell scripts.
func newDesktopNotifier() (desktopNotifier, error) {
	return newWindowsNotifier()
}

type windowsNotification struct {
	title, message string
	result         chan error
}

type windowsNotifier struct {
	requests chan windowsNotification
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

func newWindowsNotifier() (*windowsNotifier, error) {
	n := &windowsNotifier{
		requests: make(chan windowsNotification),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	ready := make(chan error, 1)
	go n.run(ready)
	if err := <-ready; err != nil {
		<-n.done
		return nil, err
	}
	return n, nil
}

func (n *windowsNotifier) show(title, message string) error {
	r := windowsNotification{title, message, make(chan error, 1)}
	select {
	case <-n.done:
		return fmt.Errorf("Windows notifications are closed")
	case n.requests <- r:
	}
	select {
	case err := <-r.result:
		return err
	case <-n.done:
		return fmt.Errorf("Windows notifications are closed")
	}
}

func (n *windowsNotifier) close() {
	n.stopOnce.Do(func() { close(n.stop) })
	<-n.done
}

// findTrayWindow returns the notification area host window used by the main
// tray, but only if it belongs to this process. A second running instance must
// never receive our notifications.
func findTrayWindow() win.HWND {
	traySearchMu.Lock()
	defer traySearchMu.Unlock()
	traySearchPID = uint32(os.Getpid())
	traySearchResult = 0
	enumWindows.Call(enumWindowsProcess, 0)
	return traySearchResult
}

// showUsingTray displays the notification over the existing tray icon so no
// second icon is created. found is false when the tray is not ready yet.
func showUsingTray(title, message string) (found, ok bool) {
	hwnd := findTrayWindow()
	if hwnd == 0 {
		return false, false
	}
	nid := win.NOTIFYICONDATA{HWnd: hwnd, UID: trayNotificationID}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.UFlags = win.NIF_INFO
	nid.DwInfoFlags = win.NIIF_INFO | win.NIIF_RESPECT_QUIET_TIME
	copyNotificationText(nid.SzInfoTitle[:], title)
	copyNotificationText(nid.SzInfo[:], message)
	return true, win.Shell_NotifyIcon(win.NIM_MODIFY, &nid)
}

// Keep the hidden window, icon and message pump on their owning OS thread.
// This also works before deej's main tray starts, or with DEEJ_NO_TRAY_ICON.
func (n *windowsNotifier) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(n.done)
	instance := win.GetModuleHandle(nil)
	hwnd := win.CreateWindowEx(0, syscall.StringToUTF16Ptr("STATIC"),
		syscall.StringToUTF16Ptr("AudioDeejSwitch notifications"),
		0, 0, 0, 0, 0, 0, 0, instance, nil)
	if hwnd == 0 {
		ready <- fmt.Errorf("create Windows notification window")
		return
	}
	defer win.DestroyWindow(hwnd)
	// LoadIcon returns a shared handle; it must not be destroyed by this host.
	appIcon := win.LoadIcon(instance, win.MAKEINTRESOURCE(2))
	if appIcon == 0 {
		appIcon = win.LoadIcon(0, win.MAKEINTRESOURCE(win.IDI_INFORMATION))
	}
	if appIcon == 0 {
		ready <- fmt.Errorf("load Windows notification icon")
		return
	}
	data := win.NOTIFYICONDATA{HWnd: hwnd, UID: 1, HIcon: appIcon}
	data.CbSize = uint32(unsafe.Sizeof(data))
	copyNotificationText(data.SzTip[:], "AudioDeejSwitch")
	active := false
	remove := func() {
		if active {
			win.Shell_NotifyIcon(win.NIM_DELETE, &data)
			active = false
		}
	}
	defer remove()
	ready <- nil
	poll := time.NewTicker(100 * time.Millisecond)
	defer poll.Stop()
	var lastShown time.Time
	for {
		select {
		case <-n.stop:
			return
		case r := <-n.requests:
			if found, ok := showUsingTray(r.title, r.message); found {
				// Never add the fallback icon while the real tray exists, even
				// if this particular notification failed.
				if !ok {
					r.result <- fmt.Errorf("show Windows notification (Shell_NotifyIcon)")
					continue
				}
				// The temporary fallback is no longer needed once the real
				// tray icon accepts notifications.
				remove()
				lastShown = time.Now()
				r.result <- nil
				continue
			}
			copyNotificationText(data.SzInfoTitle[:], r.title)
			copyNotificationText(data.SzInfo[:], r.message)
			data.UFlags = win.NIF_INFO
			data.DwInfoFlags = win.NIIF_INFO | win.NIIF_RESPECT_QUIET_TIME
			ok := active && win.Shell_NotifyIcon(win.NIM_MODIFY, &data)
			if !ok {
				// Re-add after idle cleanup or an Explorer restart.
				remove()
				data.UFlags |= win.NIF_ICON | win.NIF_TIP
				ok = win.Shell_NotifyIcon(win.NIM_ADD, &data)
			}
			active = ok
			if !ok {
				r.result <- fmt.Errorf("show Windows notification (Shell_NotifyIcon)")
				continue
			}
			lastShown = time.Now()
			r.result <- nil
		case <-poll.C:
			var msg win.MSG
			for win.PeekMessage(&msg, hwnd, 0, 0, win.PM_REMOVE) {
				win.TranslateMessage(&msg)
				win.DispatchMessage(&msg)
			}
			// A separate temporary icon allows startup errors to be shown without
			// depending on the main tray. Remove it as soon as the real tray
			// exists, or after 15 seconds without new messages.
			if active && (findTrayWindow() != 0 || time.Since(lastShown) >= 15*time.Second) {
				remove()
			}
		}
	}
}

// Native notification fields are fixed-size, NUL-terminated UTF-16 buffers.
// Truncate without leaving half of a surrogate pair, and clear previous text.
func copyNotificationText(dst []uint16, text string) {
	clear(dst)
	if len(dst) < 2 {
		return
	}
	units := utf16.Encode([]rune(strings.ReplaceAll(text, "\x00", " ")))
	n := copy(dst[:len(dst)-1], units)
	if n > 0 && dst[n-1] >= 0xD800 && dst[n-1] <= 0xDBFF {
		dst[n-1] = 0
	}
}
