package util

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/lxn/win"
	"github.com/mitchellh/go-ps"
)

const (
	getCurrentWindowInternalCooldown = time.Millisecond * 350
	getCurrentWindowMinInterval      = time.Millisecond * 50
)

type enumChildWindowsState struct {
	ownerPID uint32
	selfPID  int
	pidNames map[int]string
	result   []string
}

var (
	currentWindowCacheLock   sync.Mutex
	lastCurrentWindowHWND    win.HWND
	lastCurrentWindowResult  []string
	lastCurrentWindowCall    = time.Now()
	enumChildWindowsCallback = syscall.NewCallback(enumChildWindowsProc)

	enumChildWindowsLock sync.Mutex
	enumChildWindowsCall *enumChildWindowsState
)

// enumChildWindowsProc must be a package-level function so that its callback
// slot is allocated only once. Creating a new callback for every lookup leaks
// one of Windows' limited callback slots.
func enumChildWindowsProc(childHWND uintptr, _ uintptr) uintptr {
	state := enumChildWindowsCall
	if state == nil {
		return 1
	}

	var childPID uint32
	win.GetWindowThreadProcessId(win.HWND(childHWND), &childPID)

	if childPID == state.ownerPID || int(childPID) == state.selfPID {
		return 1
	}

	if name, ok := state.pidNames[int(childPID)]; ok && name != "" {
		state.result = append(state.result, name)
	}

	return 1
}

func getCurrentWindowProcessNames() ([]string, error) {
	hwnd := win.GetForegroundWindow()

	// apply an internal cooldown on this function to avoid calling windows API functions too frequently.
	// return a cached value during that cooldown, but only for the same window.
	now := time.Now()

	currentWindowCacheLock.Lock()
	if hwnd == lastCurrentWindowHWND && lastCurrentWindowCall.Add(getCurrentWindowInternalCooldown).After(now) {
		result := cloneStrings(lastCurrentWindowResult)
		currentWindowCacheLock.Unlock()
		return result, nil
	}
	if hwnd != lastCurrentWindowHWND && lastCurrentWindowCall.Add(getCurrentWindowMinInterval).After(now) {
		result := cloneStrings(lastCurrentWindowResult)
		currentWindowCacheLock.Unlock()
		return result, nil
	}
	currentWindowCacheLock.Unlock()

	// the logic of this implementation is a bit convoluted because of the way UWP apps
	// (also known as "modern win 10 apps" or "microsoft store apps") work.
	// these are rendered in a parent container by the name of ApplicationFrameHost.exe.
	// when windows's GetForegroundWindow is called, it returns the window owned by that parent process.
	// so whenever we get that, we need to go and look through its child windows until we find one with a different PID.
	// this behavior is most common with UWP, but it actually applies to any "container" process:
	// an acceptable approach is to return a slice of possible process names that could be the "right" one, looking
	// them up is fairly cheap and covers the most bases for apps that hide their audio-playing inside another process
	// (like steam, and the league client, and any UWP app)

	processes, err := ps.Processes()
	if err != nil {
		cacheCurrentWindowResult(hwnd, now, nil)
		return nil, fmt.Errorf("list processes: %w", err)
	}

	pidNames := make(map[int]string, len(processes))
	for _, process := range processes {
		if process == nil {
			continue
		}
		pidNames[process.Pid()] = process.Executable()
	}

	selfPID := os.Getpid()
	result := []string{}

	var ownerPID uint32
	win.GetWindowThreadProcessId(hwnd, &ownerPID)

	// check for system PID (0)
	if ownerPID == 0 {
		cacheCurrentWindowResult(hwnd, now, nil)
		return nil, nil
	}

	// add the foreground window's process, if it's still alive and isn't deej itself
	if int(ownerPID) != selfPID {
		if name, ok := pidNames[int(ownerPID)]; ok && name != "" {
			result = append(result, name)
		}
	}

	// iterate its child windows, adding their names too
	state := &enumChildWindowsState{
		ownerPID: ownerPID,
		selfPID:  selfPID,
		pidNames: pidNames,
		result:   result,
	}

	// EnumChildWindows is synchronous, so we can hand the state to the callback
	// through a package variable and serialize concurrent lookups.
	enumChildWindowsLock.Lock()
	enumChildWindowsCall = state
	win.EnumChildWindows(hwnd, enumChildWindowsCallback, 0)
	enumChildWindowsCall = nil
	enumChildWindowsLock.Unlock()

	result = uniqueStrings(state.result)

	// cache & return whichever executable names we ended up with
	cacheCurrentWindowResult(hwnd, now, result)
	return cloneStrings(result), nil
}

func cacheCurrentWindowResult(hwnd win.HWND, callTime time.Time, result []string) {
	currentWindowCacheLock.Lock()
	defer currentWindowCacheLock.Unlock()

	lastCurrentWindowHWND = hwnd
	lastCurrentWindowCall = callTime
	lastCurrentWindowResult = cloneStrings(result)
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, len(values))
	copy(result, values)
	return result
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
