package audioswitch

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

var (
	darkText     = walk.RGB(237, 237, 240)
	darkMuted    = walk.RGB(177, 177, 188)
	darkCard     = walk.RGB(38, 38, 42)
	darkInput    = walk.RGB(20, 20, 23)
	darkAccent   = walk.RGB(238, 55, 73)
	darkBorder   = walk.RGB(93, 93, 104)
	fillRectProc = syscall.NewLazyDLL("user32.dll").NewProc("FillRect")
)

type darkSurface struct {
	color walk.Color
	brush *walk.SolidColorBrush
	hbr   win.HBRUSH
}

func fillDarkRect(dc win.HDC, rect *win.RECT, brush win.HBRUSH) {
	fillRectProc.Call(uintptr(dc), uintptr(unsafe.Pointer(rect)), uintptr(brush))
}

// Walk wraps some controls in private native windows. Apply CTLCOLOR to those
// parents too, rather than only visiting the exported Walk widget tree.
func applySettingsDarkMode(mw walk.Form) error {
	surfaces := make(map[walk.Color]*darkSurface)
	dispose := func() {
		for _, s := range surfaces {
			s.brush.Dispose()
			win.DeleteObject(win.HGDIOBJ(s.hbr))
		}
	}
	createBrush := syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush")
	for _, color := range []walk.Color{pageColor, darkCard, darkInput, darkAccent, darkBorder} {
		brush, err := walk.NewSolidColorBrush(color)
		if err != nil {
			dispose()
			return err
		}
		hbr, _, _ := createBrush.Call(uintptr(color))
		if hbr == 0 {
			brush.Dispose()
			dispose()
			return fmt.Errorf("no se pudo crear el fondo del modo oscuro")
		}
		surfaces[color] = &darkSurface{color, brush, win.HBRUSH(hbr)}
	}
	mw.Disposing().Attach(dispose)
	backgrounds := make(map[win.HWND]*darkSurface)
	var visit func(walk.Window, *darkSurface)
	visit = func(w walk.Window, surface *darkSurface) {
		if w.Name() == "settings-card" {
			surface = surfaces[darkCard]
		}
		switch w.(type) {
		case *walk.ComboBox, *walk.ListBox, *walk.LineEdit, *walk.TextEdit:
			surface = surfaces[darkInput]
		}
		backgrounds[w.Handle()] = surface
		w.SetBackground(surface.brush)
		if colored, ok := w.(interface{ SetTextColor(walk.Color) }); ok {
			colored.SetTextColor(darkText)
		}
		if tabs, ok := w.(*walk.TabWidget); ok {
			for i := 0; i < tabs.Pages().Len(); i++ {
				visit(tabs.Pages().At(i), surface)
			}
		} else if container, ok := w.(walk.Container); ok {
			for i := 0; i < container.Children().Len(); i++ {
				visit(container.Children().At(i), surface)
			}
		}
	}
	visit(mw, surfaces[pageColor])
	setTheme := syscall.NewLazyDLL("uxtheme.dll").NewProc("SetWindowTheme")
	darkTheme, noTheme := syscall.StringToUTF16Ptr("DarkMode_Explorer"), syscall.StringToUTF16Ptr("")
	className := func(hwnd win.HWND) string {
		var name [128]uint16
		win.GetClassName(hwnd, &name[0], len(name))
		return syscall.UTF16ToString(name[:])
	}
	surfaceFor := func(hwnd win.HWND) *darkSurface {
		for hwnd != 0 {
			if s := backgrounds[hwnd]; s != nil {
				return s
			}
			hwnd = win.GetParent(hwnd)
		}
		return surfaces[pageColor]
	}
	install := func(hwnd win.HWND) {
		class := className(hwnd)
		theme := darkTheme
		if class == "ComboBox" || class == "SysTabControl32" {
			// These controls otherwise ignore CTLCOLOR when visual styles draw text.
			theme = noTheme
		}
		setTheme.Call(uintptr(hwnd), uintptr(unsafe.Pointer(theme)), 0)
		old := win.GetWindowLongPtr(hwnd, win.GWLP_WNDPROC)
		proc := syscall.NewCallback(func(hwnd win.HWND, msg uint32, wp, lp uintptr) uintptr {
			switch msg {
			case win.WM_CTLCOLOREDIT, win.WM_CTLCOLORLISTBOX, win.WM_CTLCOLORSTATIC, win.WM_CTLCOLORBTN:
				surface := surfaceFor(win.HWND(lp))
				if msg == win.WM_CTLCOLOREDIT || msg == win.WM_CTLCOLORLISTBOX {
					surface = surfaces[darkInput]
				}
				win.SetTextColor(win.HDC(wp), win.COLORREF(darkText))
				win.SetBkColor(win.HDC(wp), win.COLORREF(surface.color))
				return uintptr(surface.hbr)
			case win.WM_PAINT, win.WM_PRINTCLIENT:
				if class == "SysTabControl32" || class == "ComboBox" {
					var ps win.PAINTSTRUCT
					dc := win.HDC(wp)
					if msg == win.WM_PAINT {
						dc = win.BeginPaint(hwnd, &ps)
						defer win.EndPaint(hwnd, &ps)
					}
					if class == "ComboBox" {
						paintDarkCombo(hwnd, dc, surfaces)
					} else {
						paintDarkTabs(hwnd, dc, surfaces)
					}
					return 0
				}
			case win.WM_ERASEBKGND:
				if class == "SysTabControl32" || class == "ComboBox" {
					return 1
				}
			}
			return win.CallWindowProc(old, hwnd, msg, wp, lp)
		})
		win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, proc)
	}
	install(mw.Handle())
	enum := syscall.NewCallback(func(hwnd win.HWND, _ uintptr) uintptr {
		install(hwnd)
		return 1
	})
	syscall.NewLazyDLL("user32.dll").NewProc("EnumChildWindows").Call(uintptr(mw.Handle()), enum, 0)
	// DWM's dark title bar is supported on Windows 10 1809 and later.
	dwm := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	enabled := int32(1)
	if hr, _, _ := dwm.Call(uintptr(mw.Handle()), 20, uintptr(unsafe.Pointer(&enabled)), 4); hr != 0 {
		dwm.Call(uintptr(mw.Handle()), 19, uintptr(unsafe.Pointer(&enabled)), 4)
	}
	return nil
}

// A themed dropdown-list combo paints a system-white selection field even when
// CTLCOLOR returns a dark brush. Paint its face explicitly, keeping the native
// dropdown, selection, accessibility, and keyboard handling.
func paintDarkCombo(hwnd win.HWND, dc win.HDC, surfaces map[walk.Color]*darkSurface) {
	var bounds win.RECT
	win.GetClientRect(hwnd, &bounds)
	border := darkBorder
	if win.GetFocus() == hwnd {
		border = darkAccent
	}
	fillDarkRect(dc, &bounds, surfaces[border].hbr)
	bounds.Left++
	bounds.Top++
	bounds.Right--
	bounds.Bottom--
	fillDarkRect(dc, &bounds, surfaces[darkInput].hbr)
	font := win.SendMessage(hwnd, win.WM_GETFONT, 0, 0)
	previous := win.SelectObject(dc, win.HGDIOBJ(font))
	defer win.SelectObject(dc, previous)
	win.SetBkMode(dc, win.TRANSPARENT)
	win.SetTextColor(dc, win.COLORREF(darkText))
	arrowWidth := int32(24)
	if dpi := win.GetDeviceCaps(dc, win.LOGPIXELSX); dpi > 0 {
		arrowWidth = 24 * dpi / 96
	}
	arrow := bounds
	arrow.Left = arrow.Right - arrowWidth
	fillDarkRect(dc, &arrow, surfaces[darkCard].hbr)
	text := syscall.StringToUTF16Ptr("▾")
	win.DrawTextEx(dc, text, -1, &arrow, win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE, nil)
	index := win.SendMessage(hwnd, win.CB_GETCURSEL, 0, 0)
	length := int32(win.SendMessage(hwnd, win.CB_GETLBTEXTLEN, index, 0))
	if length < 0 {
		return
	}
	label := make([]uint16, int(length)+1)
	win.SendMessage(hwnd, win.CB_GETLBTEXT, index, uintptr(unsafe.Pointer(&label[0])))
	bounds.Left += 6
	bounds.Right = arrow.Left - 4
	win.DrawTextEx(dc, &label[0], length, &bounds, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS|win.DT_NOPREFIX, nil)
}

// Walk's tab painter uses system black text even on a custom dark background.
// Paint only the tab surface; the native control still owns input and selection.
func paintDarkTabs(hwnd win.HWND, dc win.HDC, surfaces map[walk.Color]*darkSurface) {
	var bounds win.RECT
	win.GetClientRect(hwnd, &bounds)
	fillDarkRect(dc, &bounds, surfaces[pageColor].hbr)
	font := win.SendMessage(hwnd, win.WM_GETFONT, 0, 0)
	previous := win.SelectObject(dc, win.HGDIOBJ(font))
	defer win.SelectObject(dc, previous)
	win.SetBkMode(dc, win.TRANSPARENT)
	selected := win.SendMessage(hwnd, win.TCM_GETCURSEL, 0, 0)
	count := win.SendMessage(hwnd, win.TCM_GETITEMCOUNT, 0, 0)
	for i := uintptr(0); i < count; i++ {
		var rect win.RECT
		win.SendMessage(hwnd, win.TCM_GETITEMRECT, i, uintptr(unsafe.Pointer(&rect)))
		color := darkMuted
		if i == selected {
			fillDarkRect(dc, &rect, surfaces[darkCard].hbr)
			underline := rect
			underline.Top = underline.Bottom - 2
			fillDarkRect(dc, &underline, surfaces[darkAccent].hbr)
			color = darkText
		}
		var text [256]uint16
		item := win.TCITEM{Mask: win.TCIF_TEXT, PszText: &text[0], CchTextMax: int32(len(text))}
		win.SendMessage(hwnd, win.TCM_GETITEM, i, uintptr(unsafe.Pointer(&item)))
		win.SetTextColor(dc, win.COLORREF(color))
		win.DrawTextEx(dc, &text[0], -1, &rect, win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_NOPREFIX, nil)
		if i == selected && win.GetFocus() == hwnd {
			rect.Left += 2
			rect.Top += 2
			rect.Right -= 2
			rect.Bottom -= 3
			win.DrawFocusRect(dc, &rect)
		}
	}
}
