package audioswitch

import (
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

const odtComboBox = 3

type outputTag struct {
	Text  string
	Color walk.Color
}

type outputEntry struct {
	Text string
	Name string
	Tags []outputTag
}

type outputCombo struct {
	walk.WidgetBase
	currentIndexChangedPublisher walk.IntEventPublisher
	entries                      []outputEntry
	brushes                      map[walk.Color]win.HBRUSH
	itemHeight                   int32
}

func newOutputCombo(parent walk.Container) (*outputCombo, error) {
	cb := &outputCombo{}
	if err := walk.InitWidget(
		cb,
		parent,
		"COMBOBOX",
		win.WS_TABSTOP|win.WS_VSCROLL|win.WS_VISIBLE|
			win.CBS_DROPDOWNLIST|win.CBS_OWNERDRAWFIXED|win.CBS_HASSTRINGS|win.CBS_NOINTEGRALHEIGHT,
		0,
	); err != nil {
		return nil, err
	}
	for _, color := range []walk.Color{darkInput, darkBorder, darkCard, darkAccent} {
		lb := win.LOGBRUSH{LbStyle: win.BS_SOLID, LbColor: win.COLORREF(color)}
		hbr := win.CreateBrushIndirect(&lb)
		if hbr == 0 {
			cb.disposeBrushes()
			return nil, syscall.GetLastError()
		}
		if cb.brushes == nil {
			cb.brushes = make(map[walk.Color]win.HBRUSH)
		}
		cb.brushes[color] = hbr
	}
	cb.itemHeight = int32(22 * cb.DPI() / 96)
	win.SendMessage(cb.Handle(), win.CB_SETITEMHEIGHT, 0, uintptr(cb.itemHeight))
	win.SendMessage(cb.Handle(), win.CB_SETITEMHEIGHT, uintptr(0xFFFFFFFF), uintptr(cb.itemHeight))
	return cb, nil
}

func (c *outputCombo) CurrentIndexChanged() *walk.IntEvent {
	return c.currentIndexChangedPublisher.Event()
}

func (c *outputCombo) CurrentIndex() int {
	return int(int32(uint32(win.SendMessage(c.Handle(), win.CB_GETCURSEL, 0, 0))))
}

func (c *outputCombo) SetCurrentIndex(index int) {
	win.SendMessage(c.Handle(), win.CB_SETCURSEL, uintptr(index), 0)
	win.InvalidateRect(c.Handle(), nil, true)
}

func (c *outputCombo) SetItems(entries []outputEntry) {
	c.entries = entries
	hwnd := c.Handle()
	win.SendMessage(hwnd, win.CB_RESETCONTENT, 0, 0)
	for _, entry := range entries {
		text, err := syscall.UTF16PtrFromString(entry.Text)
		if err != nil {
			continue
		}
		win.SendMessage(hwnd, win.CB_ADDSTRING, 0, uintptr(unsafe.Pointer(text)))
	}
	win.InvalidateRect(hwnd, nil, true)
}

func (c *outputCombo) Dispose() {
	c.disposeBrushes()
	c.WidgetBase.Dispose()
}

func (c *outputCombo) disposeBrushes() {
	for _, hbr := range c.brushes {
		win.DeleteObject(win.HGDIOBJ(hbr))
	}
	c.brushes = nil
}

func (c *outputCombo) CreateLayoutItem(ctx *walk.LayoutContext) walk.LayoutItem {
	scale := ctx.DPI()
	return &outputComboLayoutItem{
		idealSize: walk.Size{Width: 300 * scale / 96, Height: 25 * scale / 96},
	}
}

type outputComboLayoutItem struct {
	walk.LayoutItemBase
	idealSize walk.Size
}

func (li *outputComboLayoutItem) LayoutFlags() walk.LayoutFlags {
	return walk.GrowableHorz
}

func (li *outputComboLayoutItem) IdealSize() walk.Size {
	return li.idealSize
}

func (li *outputComboLayoutItem) MinSize() walk.Size {
	return li.idealSize
}

func messagePointer(value uintptr) unsafe.Pointer {
	var pointer unsafe.Pointer
	*(*uintptr)(unsafe.Pointer(&pointer)) = value
	return pointer
}

func (c *outputCombo) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		dc := win.BeginPaint(c.Handle(), &ps)
		c.paintFace(dc)
		win.EndPaint(c.Handle(), &ps)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_DRAWITEM:
		dis := (*win.DRAWITEMSTRUCT)(messagePointer(lParam))
		if dis.CtlType == odtComboBox {
			c.drawItem(dis)
			return win.TRUE
		}
	case win.WM_MEASUREITEM:
		mis := (*win.MEASUREITEMSTRUCT)(messagePointer(lParam))
		if mis.CtlType == odtComboBox {
			mis.ItemHeight = uint32(c.itemHeight)
			return win.TRUE
		}
	case win.WM_COMMAND:
		if win.HIWORD(uint32(wParam)) == win.CBN_SELCHANGE {
			win.InvalidateRect(c.Handle(), nil, true)
			c.currentIndexChangedPublisher.Publish(c.CurrentIndex())
			return 0
		}
	}
	return c.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}

func (c *outputCombo) paintFace(dc win.HDC) {
	var bounds win.RECT
	win.GetClientRect(c.Handle(), &bounds)
	border := darkBorder
	if win.GetFocus() == c.Handle() {
		border = darkAccent
	}
	fillDarkRect(dc, &bounds, c.brushes[border])
	bounds.Left++
	bounds.Top++
	bounds.Right--
	bounds.Bottom--
	fillDarkRect(dc, &bounds, c.brushes[darkInput])

	arrowWidth := int32(24)
	if dpi := win.GetDeviceCaps(dc, win.LOGPIXELSX); dpi > 0 {
		arrowWidth = 24 * dpi / 96
	}
	arrow := bounds
	arrow.Left = arrow.Right - arrowWidth
	fillDarkRect(dc, &arrow, c.brushes[darkCard])
	win.SetBkMode(dc, win.TRANSPARENT)
	win.SetTextColor(dc, win.COLORREF(darkText))
	win.DrawTextEx(dc, syscall.StringToUTF16Ptr("▾"), -1, &arrow, win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE, nil)

	index := c.CurrentIndex()
	if index < 0 || index >= len(c.entries) {
		return
	}
	textRect := bounds
	textRect.Left += 6
	textRect.Right = arrow.Left - 4
	font := win.SendMessage(c.Handle(), win.WM_GETFONT, 0, 0)
	previous := win.SelectObject(dc, win.HGDIOBJ(font))
	defer win.SelectObject(dc, previous)
	c.drawEntryText(dc, &textRect, c.entries[index])
}

func (c *outputCombo) drawItem(dis *win.DRAWITEMSTRUCT) {
	dc := dis.HDC
	rect := dis.RcItem
	background := darkInput
	if dis.ItemState&win.ODS_COMBOBOXEDIT == 0 {
		if dis.ItemState&win.ODS_SELECTED != 0 {
			background = darkBorder
		}
	} else if dis.ItemState&win.ODS_DISABLED != 0 {
		background = darkCard
	}
	if brush := c.brushes[background]; brush != 0 {
		fillDarkRect(dc, &rect, brush)
	}
	int32Index := int(dis.ItemID)
	if int32Index < 0 || int32Index >= len(c.entries) {
		return
	}
	font := win.SendMessage(c.Handle(), win.WM_GETFONT, 0, 0)
	previous := win.SelectObject(dc, win.HGDIOBJ(font))
	defer win.SelectObject(dc, previous)
	win.SetBkMode(dc, win.TRANSPARENT)
	textRect := rect
	textRect.Left += 6
	textRect.Right -= 4
	c.drawEntryText(dc, &textRect, c.entries[int32Index])
	if dis.ItemState&win.ODS_FOCUS != 0 && dis.ItemState&win.ODS_COMBOBOXEDIT == 0 {
		focus := textRect
		focus.Left -= 2
		focus.Right += 2
		win.DrawFocusRect(dc, &focus)
	}
}

func (c *outputCombo) drawEntryText(dc win.HDC, rect *win.RECT, entry outputEntry) {
	segments := []outputTag{{Text: entry.Name, Color: darkText}}
	for _, tag := range entry.Tags {
		segments = append(segments, outputTag{Text: " [" + tag.Text + "]", Color: tag.Color})
	}
	x := rect.Left
	for _, segment := range segments {
		ptr := syscall.StringToUTF16Ptr(segment.Text)
		measure := win.RECT{}
		win.DrawTextEx(dc, ptr, -1, &measure, win.DT_CALCRECT|win.DT_SINGLELINE|win.DT_NOPREFIX, nil)
		width := measure.Right - measure.Left
		if x >= rect.Right {
			break
		}
		draw := win.RECT{Left: x, Top: rect.Top, Right: rect.Right, Bottom: rect.Bottom}
		win.SetTextColor(dc, win.COLORREF(segment.Color))
		win.DrawTextEx(dc, ptr, -1, &draw, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_NOPREFIX, nil)
		x += width
	}
}

type outputComboDeclaration struct {
	AssignTo    **outputCombo
	Font        declarative.Font
	MinSize     declarative.Size
	MaxSize     declarative.Size
	ToolTipText declarative.Property
	Enabled     declarative.Property
	Visible     declarative.Property
}

func (d outputComboDeclaration) Create(builder *declarative.Builder) error {
	cb, err := newOutputCombo(builder.Parent())
	if err != nil {
		return err
	}
	if d.AssignTo != nil {
		*d.AssignTo = cb
	}
	return builder.InitWidget(d, cb, nil)
}
