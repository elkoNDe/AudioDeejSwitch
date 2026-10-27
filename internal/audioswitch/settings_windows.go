package audioswitch

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	ole "github.com/go-ole/go-ole"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"gopkg.in/yaml.v2"
)

var (
	pageColor = walk.RGB(28, 28, 30)
)

// Settings runs in a child process: Walk and systray each own their UI thread.
// Closing this window cannot stop the mixer or its global shortcuts.
func RunSettings() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if e, ok := err.(*ole.OleError); !ok || e.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()
	userConfig, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	preferencesPath := filepath.Join(userConfig, "AudioDeejSwitch", "preferences.json")
	preferences, err := loadSettingsPreferences(preferencesPath)
	if err != nil {
		return err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("%s: %w", ConfigPath, err)
	}
	switchOriginal, err := os.ReadFile(ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	original, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}
	var mw *walk.MainWindow
	var outputs *walk.ComboBox
	var shortcuts *walk.ListBox
	var hotkey *walk.LineEdit
	var editor *walk.TextEdit
	var status *walk.Label
	var devices []Device
	capturedHotkey := ""
	var appIcon *walk.Icon
	if icon, err := walk.NewIconFromResourceId(2); err == nil {
		appIcon = icon
	} else if icon, err := walk.NewIconFromResourceIdWithSize(2, walk.Size{Width: 32, Height: 32}); err == nil {
		appIcon = icon
	} else if icon, err := walk.NewIconFromFile("assets/deej-red-black.ico"); err == nil {
		appIcon = icon
	}
	var capture hotkeyCapture
	capture.onKey = func(mods, key uint32) {
		if err := ValidateHotkey(mods, key); err != nil {
			hotkey.SetText(err.Error())
			capture.begin()
			return
		}
		capturedHotkey = FormatHotkey(mods, key)
		hotkey.SetText(capturedHotkey)
	}
	capture.onCancel = func() { hotkey.SetText(capturedHotkey) }
	showError := func(err error) { walk.MsgBox(mw, "AudioDeejSwitch", err.Error(), walk.MsgBoxIconError) }
	refreshRules := func() {
		labels := []string{}
		for _, b := range cfg.Bindings {
			labels = append(labels, b.Hotkey+" → "+b.Name)
		}
		shortcuts.SetModel(labels)
	}
	refreshDevices := func() {
		var err error
		devices, err = Devices()
		if err != nil {
			showError(err)
			return
		}
		labels := []string{"Siguiente salida disponible (alternar)"}
		current := "Sin salida predeterminada"
		for _, d := range devices {
			label := d.Name
			if d.Default {
				current = d.Name
				label += " [actual]"
			}
			labels = append(labels, label)
		}
		outputs.SetModel(labels)
		outputs.SetCurrentIndex(0)
		status.SetText("Salida actual: " + current)
	}
	selected := func() (Binding, bool) {
		i := outputs.CurrentIndex()
		if i < 0 || i > len(devices) {
			return Binding{}, false
		}
		if i == 0 {
			return Binding{DeviceID: "cycle", Name: "Siguiente salida"}, true
		}
		return Binding{DeviceID: devices[i-1].ID, Name: devices[i-1].Name}, true
	}
	dirtyRules := false
	saveRules := func() {
		cfg.Communications = false
		if err := cfg.Validate(); err != nil {
			showError(err)
			return
		}
		data, err := yaml.Marshal(cfg)
		if err == nil {
			err = SaveWithBackup(ConfigPath, switchOriginal, data)
		}
		if err != nil {
			showError(err)
			return
		}
		switchOriginal = data
		dirtyRules = false
		if !preferences.HideHotkeysSaved {
			hide, err := showHotkeysSaved(mw)
			if err != nil {
				showError(err)
				return
			}
			if hide {
				candidate := preferences
				candidate.HideHotkeysSaved = true
				if err := saveSettingsPreferences(preferencesPath, candidate); err != nil {
					showError(err)
					return
				}
				preferences = candidate
			}
		}
	}
	err = (MainWindow{
		AssignTo: &mw,
		Title:    "Audio Deej Switch — Configuración",
		Icon:     appIcon,
		Font:     Font{Family: "Segoe UI", PointSize: 9},
		MinSize:  Size{Width: 720, Height: 580},
		Size:     Size{Width: 780, Height: 660},
		Layout:   VBox{MarginsZero: true},
		Children: []Widget{
			Composite{
				Layout:     VBox{Margins: Margins{Left: 20, Top: 14, Right: 20, Bottom: 14}, Spacing: 12},
				Background: SolidColorBrush{Color: pageColor},
				Children: []Widget{
					TabWidget{
						Pages: []TabPage{
							{
								Title:  "Salidas y atajos",
								Layout: VBox{Margins: Margins{Left: 18, Top: 16, Right: 18, Bottom: 18}, Spacing: 12},
								Children: []Widget{
									Composite{
										Name:   "settings-card",
										Layout: VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 14}, Spacing: 10},
										Children: []Widget{
											Label{Text: "Salida actual", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
											Label{AssignTo: &status, Text: "Salidas de audio", Font: Font{Family: "Segoe UI", PointSize: 9}},
											ComboBox{AssignTo: &outputs},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Usar esta salida", OnClicked: func() {
													b, ok := selected()
													if !ok {
														return
													}
													if _, err := Switch(b.DeviceID, false); err != nil {
														showError(err)
														return
													}
													refreshDevices()
												}},
												PushButton{Text: "Actualizar dispositivos", OnClicked: refreshDevices},
												HSpacer{},
											}},
										},
									},
									Composite{
										Name:          "settings-card",
										StretchFactor: 1,
										Layout:        VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 14}, Spacing: 10},
										Children: []Widget{
											Label{Text: "Atajos globales (Esc para cancelar)", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												LineEdit{AssignTo: &hotkey, ReadOnly: true, CueBanner: "Sin atajo", StretchFactor: 1},
												PushButton{Text: "Capturar", OnClicked: func() {
													capture.begin()
													hotkey.SetText("Pulsa la combinación…")
													if err := hotkey.SetFocus(); err != nil {
														capture.stop()
														showError(err)
													}
												}},
												PushButton{Text: "Añadir atajo", OnClicked: func() {
													b, ok := selected()
													if !ok {
														return
													}
													if capturedHotkey == "" {
														showError(fmt.Errorf("captura un atajo primero"))
														return
													}
													b.Hotkey = capturedHotkey
													candidate := cfg
													candidate.Bindings = append(append([]Binding{}, cfg.Bindings...), b)
													if err := candidate.Validate(); err != nil {
														showError(err)
														return
													}
													cfg = candidate
													dirtyRules = true
													refreshRules()
													capturedHotkey = ""
													capture.stop()
													hotkey.SetText("")
												}},
											}},
											Label{Text: "Lista de atajos configurados:", Font: Font{Family: "Segoe UI", PointSize: 9}},
											ListBox{AssignTo: &shortcuts, StretchFactor: 1},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Eliminar seleccionado", OnClicked: func() {
													i := shortcuts.CurrentIndex()
													if i < 0 {
														return
													}
													cfg.Bindings = append(cfg.Bindings[:i], cfg.Bindings[i+1:]...)
													dirtyRules = true
													refreshRules()
												}},
												HSpacer{},
												PushButton{Text: "Guardar atajos", OnClicked: saveRules},
											}},
											Label{Text: "Los atajos funcionan con esta ventana cerrada. Cerrar esta venta no cierra el programa.", Font: Font{Family: "Segoe UI", PointSize: 8}, TextColor: walk.RGB(90, 90, 90)},
										},
									},
								},
							},
							{
								Title:  "deej · config.yaml",
								Layout: VBox{Margins: Margins{Left: 18, Top: 16, Right: 18, Bottom: 18}, Spacing: 12},
								Children: []Widget{
									Composite{
										Name:          "settings-card",
										StretchFactor: 1,
										Layout:        VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 14}, Spacing: 10},
										Children: []Widget{
											Label{Text: "Configuración avanzada", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
											Label{Text: "Edita tu configuración habitual de deej. Se valida y crea config.yaml.bak antes de guardar.", Font: Font{Family: "Segoe UI", PointSize: 9}},
											TextEdit{AssignTo: &editor, Text: strings.ReplaceAll(strings.ReplaceAll(string(original), "\r\n", "\n"), "\n", "\r\n"), HScroll: true, VScroll: true, Font: Font{Family: "Consolas", PointSize: 9}, StretchFactor: 1},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Recargar del archivo", OnClicked: func() {
													if walk.MsgBox(mw, "Recargar", "¿Descartar los cambios del editor y leer config.yaml?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
														return
													}
													b, err := os.ReadFile("config.yaml")
													if err != nil {
														showError(err)
														return
													}
													original = b
													editor.SetText(strings.ReplaceAll(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n", "\r\n"))
												}},
												HSpacer{},
												PushButton{Text: "Validar y guardar YAML", OnClicked: func() {
													b := []byte(strings.ReplaceAll(editor.Text(), "\r\n", "\n"))
													if err := ValidateDeejYAML(b); err != nil {
														showError(err)
														return
													}
													if err := SaveWithBackup("config.yaml", original, b); err != nil {
														showError(err)
														return
													}
													original = b
													walk.MsgBox(mw, "Configuración guardada", "deej recargará el archivo automáticamente si está en ejecución.", walk.MsgBoxIconInformation)
												}},
											}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}).Create()
	if err != nil {
		return err
	}
	defer mw.Dispose()
	if err := applySettingsDarkMode(mw); err != nil {
		return err
	}
	if appIcon != nil {
		mw.SetIcon(appIcon)
	}
	if err := capture.install(hotkey.Handle()); err != nil {
		return err
	}
	defer capture.uninstall()
	hotkey.EditingFinished().Attach(func() {
		if capture.active {
			capture.stop()
			hotkey.SetText(capturedHotkey)
		}
	})
	refreshDevices()
	refreshRules()
	dirtyRules = false
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		text := strings.ReplaceAll(editor.Text(), "\r\n", "\n")
		if dirtyRules || text != strings.ReplaceAll(string(original), "\r\n", "\n") {
			*canceled = walk.MsgBox(mw, "Cambios sin guardar", "¿Cerrar y descartar los cambios sin guardar?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes
		}
	})
	mw.Run()
	return nil
}

type hotkeyCapture struct {
	hwnd         win.HWND
	oldProc      uintptr
	proc         uintptr
	active       bool
	suppressChar bool
	onKey        func(mods, key uint32)
	onCancel     func()
}

func (c *hotkeyCapture) begin() {
	c.active = true
}

func (c *hotkeyCapture) stop() {
	c.active = false
	c.suppressChar = false
}

func (c *hotkeyCapture) install(hwnd win.HWND) error {
	c.hwnd = hwnd
	c.oldProc = win.GetWindowLongPtr(hwnd, win.GWLP_WNDPROC)
	if c.oldProc == 0 {
		return fmt.Errorf("no se pudo preparar la captura del atajo")
	}
	c.proc = syscall.NewCallback(c.wndProc)
	if win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, c.proc) == 0 {
		c.proc = 0
		return fmt.Errorf("no se pudo preparar la captura del atajo")
	}
	return nil
}

func (c *hotkeyCapture) uninstall() {
	if c.proc == 0 || c.hwnd == 0 {
		return
	}
	win.SetWindowLongPtr(c.hwnd, win.GWLP_WNDPROC, c.oldProc)
	c.proc = 0
}

func (c *hotkeyCapture) wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == win.WM_NCDESTROY {
		c.proc = 0
	}
	switch msg {
	case win.WM_CHAR, win.WM_SYSCHAR, win.WM_DEADCHAR:
		if c.suppressChar {
			return 0
		}
	case win.WM_KEYUP, win.WM_SYSKEYUP:
		c.suppressChar = false
	}
	if c.active && (msg == win.WM_KEYDOWN || msg == win.WM_SYSKEYDOWN) {
		key := uint32(wParam)
		c.suppressChar = true
		if key == win.VK_ESCAPE {
			c.active = false
			if c.onCancel != nil {
				c.onCancel()
			}
			return 0
		}
		if isModifierKey(key) {
			return 0
		}
		c.active = false
		mods := currentModifiers()
		if c.onKey != nil {
			c.onKey(mods, key)
		}
		return 0
	}
	return win.CallWindowProc(c.oldProc, hwnd, msg, wParam, lParam)
}

func currentModifiers() uint32 {
	var mods uint32
	if keyDown(win.VK_MENU) {
		mods |= modAlt
	}
	if keyDown(win.VK_CONTROL) {
		mods |= modCtrl
	}
	if keyDown(win.VK_SHIFT) {
		mods |= modShift
	}
	if keyDown(win.VK_LWIN) || keyDown(win.VK_RWIN) {
		mods |= modWin
	}
	return mods
}

func keyDown(vk int32) bool {
	return uint16(win.GetKeyState(vk))&0x8000 != 0
}
