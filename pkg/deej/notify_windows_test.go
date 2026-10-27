package deej

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
)

func TestNotificationText(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		size              int
	}{
		{"native text", "Atajo: Ctrl+Alt+F8 — salida", "Atajo: Ctrl+Alt+F8 — salida", 64},
		{"script characters stay text", "$(exit); ' ` \"", "$(exit); ' ` \"", 64},
		{"embedded NUL", "uno\x00dos", "uno dos", 16},
		{"terminated truncation", strings.Repeat("a", 64), strings.Repeat("a", 63), 64},
		{"complete surrogate", "a🎧", "a🎧", 4},
		{"split surrogate", "aa🎧", "aa", 4},
		{"empty buffer", "texto", "", 0},
		{"terminator only", "texto", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]uint16, tc.size)
			for i := range buf {
				buf[i] = 'x'
			}
			copyNotificationText(buf, tc.input)
			if got := syscall.UTF16ToString(buf); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if len(buf) > 0 && buf[len(buf)-1] != 0 {
				t.Fatal("missing NUL terminator")
			}
			for _, r := range utf16.Decode(buf) {
				if r == '\uFFFD' {
					t.Fatal("broken surrogate pair")
				}
			}
			copyNotificationText(buf, "")
			for _, c := range buf {
				if c != 0 {
					t.Fatal("previous notification text was not cleared")
				}
			}
		})
	}
}

func TestWindowsNotifierLifecycle(t *testing.T) {
	n, err := newWindowsNotifier()
	if err != nil {
		t.Fatal(err)
	}
	defer n.close()
	// Opt in to a real desktop notification; normal tests only create and
	// destroy the hidden native host without showing anything to the user.
	if os.Getenv("AUDIODEEJSWITCH_TEST_NOTIFICATIONS") == "1" {
		if err := n.show("AudioDeejSwitch: prueba de notificaciones",
			"Aviso nativo de Windows. No cambia el audio ni utiliza PowerShell."); err != nil {
			t.Fatal(err)
		}
	}
	closed := make(chan struct{})
	go func() { n.close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("native notification host did not stop")
	}
	if err := n.show("closed", "must not block after shutdown"); err == nil {
		t.Fatal("notification accepted after shutdown")
	}
}
