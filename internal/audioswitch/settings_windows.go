package audioswitch

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	ole "github.com/go-ole/go-ole"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"gopkg.in/yaml.v2"
)

var (
	pageColor     = walk.RGB(28, 28, 30)
	showScrollBar = syscall.NewLazyDLL("user32.dll").NewProc("ShowScrollBar")
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
	initialBaud := 9600
	if value, ok := GetTopLevelScalar(string(original), "baud_rate"); ok {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			initialBaud = parsed
		}
	}
	initialNoise := "default"
	if value, ok := GetTopLevelScalar(string(original), "noise_reduction"); ok {
		switch strings.ToLower(value) {
		case "low", "default", "high":
			initialNoise = strings.ToLower(value)
		}
	}
	initialNoiseIndex := 1
	switch initialNoise {
	case "low":
		initialNoiseIndex = 0
	case "high":
		initialNoiseIndex = 2
	}
	var mw *walk.MainWindow
	var outputs *walk.ComboBox
	var shortcuts *walk.ListBox
	var hotkey *walk.LineEdit
	var startupButton *walk.PushButton
	var autoStartButton *walk.PushButton
	var portsCombo *walk.ComboBox
	var baudRate *walk.NumberEdit
	var noiseCombo *walk.ComboBox
	var connectionNote *walk.Label
	var abButton *walk.PushButton
	var abPanel *walk.Composite
	var outputA *walk.ComboBox
	var outputB *walk.ComboBox
	var slidersList *walk.ListBox
	var sliderIndex *walk.LineEdit
	var sliderTarget *walk.ComboBox
	var visualNote *walk.Label
	var sliderEntries []SliderMappingEntry
	var savedNote *walk.Label
	var devices []Device
	var abEnabled bool
	loadedOutputIndex := 0
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
	var noiseLevelForIndex func(int) string
	var noiseIndexForLevel func(string) int
	var setABMode func(bool)
	var updateAutoStartButton func()
	var refreshPorts func()
	var refreshConnectionFields func()
	var reloadDeejFile func()
	var refreshABCombos func()
	var selectedToggle func() (ToggleTarget, ToggleTarget, bool)
	var abIndexForDevice func(string) int
	var persistRules func() bool
	var saveDeejConnection func()
	var loadVisualMapping func()
	var refreshSliderList func()
	var saveVisualMapping func()
	var applySliderEntry func()
	var removeSliderEntry func()
	var refreshSliderTargets func()
	var splitSliderTargets func(string) []string
	var refreshRules func()
	var selected func() (Binding, bool)
	var updateStartupButton func()
	var refreshDevices func()
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
	noiseLevelForIndex = func(index int) string {
		switch index {
		case 0:
			return "low"
		case 2:
			return "high"
		default:
			return "default"
		}
	}
	noiseIndexForLevel = func(level string) int {
		switch strings.ToLower(level) {
		case "low":
			return 0
		case "high":
			return 2
		default:
			return 1
		}
	}
	setABMode = func(enabled bool) {
		abEnabled = enabled
		if abButton != nil {
			if enabled {
				abButton.SetText("Ocultar")
			} else {
				abButton.SetText("Alternar")
			}
		}
		if abPanel != nil {
			abPanel.SetVisible(enabled)
		}
		if updateStartupButton != nil {
			updateStartupButton()
		}
	}
	updateAutoStartButton = func() {
		if autoStartButton == nil {
			return
		}
		enabled, err := StartupEnabled()
		if err == nil && enabled {
			autoStartButton.SetText("Quitar inicio de Windows")
		} else {
			autoStartButton.SetText("Iniciar con Windows")
		}
	}
	refreshPorts = func() {
		if portsCombo == nil {
			return
		}
		ports, err := AvailableCOMPorts()
		if err != nil {
			showError(err)
			return
		}
		configured, _ := GetTopLevelScalar(string(original), "com_port")
		if configured != "" {
			found := false
			for _, p := range ports {
				if p == configured {
					found = true
					break
				}
			}
			if !found {
				ports = append([]string{configured}, ports...)
			}
		}
		portsCombo.SetModel(ports)
		if configured != "" {
			portsCombo.SetText(configured)
		} else if len(ports) > 0 {
			portsCombo.SetCurrentIndex(0)
		}
	}
	refreshConnectionFields = func() {
		if portsCombo == nil || baudRate == nil || noiseCombo == nil {
			return
		}
		refreshPorts()
		if baud, ok := GetTopLevelScalar(string(original), "baud_rate"); ok {
			if parsed, err := strconv.Atoi(baud); err == nil && parsed > 0 {
				baudRate.SetValue(float64(parsed))
			}
		}
		if noise, ok := GetTopLevelScalar(string(original), "noise_reduction"); ok {
			noiseCombo.SetCurrentIndex(noiseIndexForLevel(noise))
		}
	}
	reloadDeejFile = func() {
		b, err := os.ReadFile("config.yaml")
		if err != nil {
			showError(err)
			return
		}
		original = b
		loadVisualMapping()
		refreshConnectionFields()
		if visualNote != nil {
			visualNote.SetText("config.yaml recargado desde disco.")
		}
	}
	refreshABCombos = func() {
		if outputA == nil || outputB == nil {
			return
		}
		previousA, previousB := outputA.CurrentIndex(), outputB.CurrentIndex()
		labels := make([]string, 0, len(devices))
		for _, d := range devices {
			labels = append(labels, d.Name)
		}
		outputA.SetModel(labels)
		outputB.SetModel(labels)
		if len(labels) == 0 {
			outputA.SetCurrentIndex(-1)
			outputB.SetCurrentIndex(-1)
			return
		}
		if previousA < 0 || previousA >= len(labels) {
			previousA = 0
		}
		if previousB < 0 || previousB >= len(labels) {
			previousB = 0
			if len(labels) > 1 {
				previousB = 1
			}
		}
		outputA.SetCurrentIndex(previousA)
		outputB.SetCurrentIndex(previousB)
	}
	selectedToggle = func() (ToggleTarget, ToggleTarget, bool) {
		if outputA == nil || outputB == nil {
			return ToggleTarget{}, ToggleTarget{}, false
		}
		i, j := outputA.CurrentIndex(), outputB.CurrentIndex()
		if i < 0 || j < 0 || i >= len(devices) || j >= len(devices) || i == j {
			return ToggleTarget{}, ToggleTarget{}, false
		}
		return ToggleTarget{DeviceID: devices[i].ID, Name: devices[i].Name},
			ToggleTarget{DeviceID: devices[j].ID, Name: devices[j].Name}, true
	}
	abIndexForDevice = func(id string) int {
		for i, d := range devices {
			if d.ID == id {
				return i
			}
		}
		return -1
	}
	saveDeejConnection = func() {
		port := strings.TrimSpace(portsCombo.Text())
		baud := int(baudRate.Value())
		if baud <= 0 {
			showError(fmt.Errorf("la velocidad de conexi\u00f3n debe ser mayor que 0"))
			return
		}
		noiseIndex := noiseCombo.CurrentIndex()
		if noiseIndex < 0 || noiseIndex > 2 {
			showError(fmt.Errorf("elige un nivel de suavizado"))
			return
		}
		noise := noiseLevelForIndex(noiseIndex)
		text := string(original)
		if port != "" {
			text = SetTopLevelScalar(text, "com_port", port)
		}
		text = SetTopLevelScalar(text, "baud_rate", strconv.Itoa(baud))
		text = SetTopLevelScalar(text, "noise_reduction", noise)
		data := []byte(strings.ReplaceAll(text, "\r\n", "\n"))
		if err := ValidateDeejYAML(data); err != nil {
			showError(err)
			return
		}
		if err := SaveWithBackup("config.yaml", original, data); err != nil {
			showError(err)
			return
		}
		original = data
		if connectionNote != nil {
			connectionNote.SetText("Conexi\u00f3n guardada en config.yaml.")
		}
	}
	loadVisualMapping = func() {
		entries, err := ParseSliderMapping(string(original))
		if err != nil {
			showError(err)
			return
		}
		sliderEntries = entries
		refreshSliderList()
		sliderIndex.SetText("")
		sliderTarget.SetText("")
		refreshSliderTargets()
		if visualNote != nil {
			visualNote.SetText(fmt.Sprintf("%d sliders cargados desde config.yaml.", len(entries)))
		}
	}
	refreshSliderList = func() {
		if slidersList == nil {
			return
		}
		labels := make([]string, len(sliderEntries))
		for i, e := range sliderEntries {
			labels[i] = fmt.Sprintf("Slider %d: %s", e.Index, strings.Join(e.Targets, ", "))
		}
		slidersList.SetModel(labels)
		updateShortcutScrollbar(slidersList)
	}
	saveVisualMapping = func() {
		if len(sliderEntries) == 0 {
			showError(fmt.Errorf("no hay sliders que guardar"))
			return
		}
		text, err := ReplaceSliderMapping(string(original), sliderEntries)
		if err != nil {
			showError(err)
			return
		}
		data := []byte(strings.ReplaceAll(text, "\r\n", "\n"))
		if err := ValidateDeejYAML(data); err != nil {
			showError(err)
			return
		}
		if err := SaveWithBackup("config.yaml", original, data); err != nil {
			showError(err)
			return
		}
		original = data
		if visualNote != nil {
			visualNote.SetText("Mapping guardado en config.yaml.")
		}
	}
	applySliderEntry = func() {
		if slidersList == nil {
			return
		}
		index, err := strconv.Atoi(strings.TrimSpace(sliderIndex.Text()))
		if err != nil || index < 0 {
			showError(fmt.Errorf("\u00edndice inv\u00e1lido: usa un n\u00famero mayor o igual que 0"))
			return
		}
		targets := splitSliderTargets(sliderTarget.Text())
		if len(targets) == 0 {
			showError(fmt.Errorf("escribe al menos un destino; separa varios con comas"))
			return
		}
		replaced := false
		for i := range sliderEntries {
			if sliderEntries[i].Index == index {
				sliderEntries[i].Targets = targets
				replaced = true
				break
			}
		}
		if !replaced {
			sliderEntries = append(sliderEntries, SliderMappingEntry{Index: index, Targets: targets})
		}
		sort.Slice(sliderEntries, func(i, j int) bool { return sliderEntries[i].Index < sliderEntries[j].Index })
		refreshSliderList()
		if visualNote != nil {
			visualNote.SetText(fmt.Sprintf("Slider %d listo; pulsa Guardar para aplicarlo.", index))
		}
	}
	removeSliderEntry = func() {
		if slidersList == nil {
			return
		}
		i := slidersList.CurrentIndex()
		if i < 0 || i >= len(sliderEntries) {
			return
		}
		sliderEntries = append(append([]SliderMappingEntry{}, sliderEntries[:i]...), sliderEntries[i+1:]...)
		refreshSliderList()
		sliderIndex.SetText("")
		sliderTarget.SetText("")
	}
	refreshSliderTargets = func() {
		if sliderTarget == nil {
			return
		}
		apps := RunningAudioApps()
		defaults := []string{"master", "mic", "system", "deej.current", "deej.unmapped"}
		seen := make(map[string]bool)
		var model []string
		for _, d := range defaults {
			if !seen[d] {
				model = append(model, d)
				seen[d] = true
			}
		}
		for _, a := range apps {
			if !seen[a] {
				model = append(model, a)
				seen[a] = true
			}
		}
		for _, e := range sliderEntries {
			for _, t := range e.Targets {
				if !seen[t] {
					model = append(model, t)
					seen[t] = true
				}
			}
		}
		sliderTarget.SetModel(model)
	}
	splitSliderTargets = func(text string) []string {
		parts := strings.Split(text, ",")
		targets := make([]string, 0, len(parts))
		for _, part := range parts {
			if target := strings.TrimSpace(part); target != "" {
				targets = append(targets, target)
			}
		}
		return targets
	}
	refreshRules = func() {
		labels := []string{}
		for _, b := range cfg.Bindings {
			labels = append(labels, b.Hotkey+" → "+b.DisplayName())
		}
		shortcuts.SetModel(labels)
		updateShortcutScrollbar(shortcuts)
	}
	selected = func() (Binding, bool) {
		if outputs == nil {
			return Binding{}, false
		}
		i := outputs.CurrentIndex()
		if i < 0 || i > len(devices) {
			return Binding{}, false
		}
		if i == 0 {
			return Binding{DeviceID: "cycle", Name: "Siguiente salida"}, true
		}
		b := Binding{DeviceID: devices[i-1].ID, Name: devices[i-1].Name}
		return b, true
	}
	updateStartupButton = func() {
		if startupButton == nil {
			return
		}
		if abEnabled {
			startupButton.SetEnabled(false)
			startupButton.SetText("Al iniciar")
			return
		}
		startupButton.SetEnabled(true)
		b, ok := selected()
		if ok && b.DeviceID != "cycle" && cfg.Startup != nil && cfg.Startup.DeviceID == b.DeviceID {
			startupButton.SetText("Quitar inicio")
		} else {
			startupButton.SetText("Al iniciar")
		}
	}
	refreshDevices = func() {
		var err error
		devices, err = Devices()
		if err != nil {
			showError(err)
			return
		}
		previousIndex := outputs.CurrentIndex()
		labels := []string{"Siguiente salida disponible (alternar)"}
		for _, d := range devices {
			label := d.Name
			tags := []string{}
			if d.Default {
				tags = append(tags, "actual")
			}
			if cfg.Startup != nil && cfg.Startup.DeviceID == d.ID {
				tags = append(tags, "inicio")
			}
			if len(tags) > 0 {
				label += " [" + strings.Join(tags, ", ") + "]"
			}
			labels = append(labels, label)
		}
		outputs.SetModel(labels)
		if previousIndex >= 0 && previousIndex < len(labels) {
			outputs.SetCurrentIndex(previousIndex)
		} else {
			outputs.SetCurrentIndex(0)
		}
		updateStartupButton()
		refreshABCombos()
	}
	selectedShortcut := func() (int, bool) {
		i := shortcuts.CurrentIndex()
		return i, i >= 0 && i < len(cfg.Bindings)
	}
	outputIndexForDevice := func(id string) int {
		for i, d := range devices {
			if d.ID == id {
				return i + 1
			}
		}
		return 0
	}
	persistRules = func() bool {
		cfg.Communications = false
		if err := cfg.Validate(); err != nil {
			showError(err)
			return false
		}
		data, err := yaml.Marshal(cfg)
		if err == nil {
			err = SaveWithBackup(ConfigPath, switchOriginal, data)
		}
		if err != nil {
			showError(err)
			return false
		}
		switchOriginal = data
		if savedNote != nil {
			savedNote.SetText("Atajos guardados.")
		}
		return true
	}
	err = (MainWindow{
		AssignTo: &mw,
		Title:    "Audio Deej Switch — Configuración",
		Icon:     appIcon,
		Font:     Font{Family: "Segoe UI", PointSize: 9},
		Size:     Size{Width: 780, Height: 720},
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
											Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
												Label{Text: "Salidas", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
												HSpacer{},
												PushButton{
													Text:        "⟳",
													ToolTipText: "Actualizar dispositivos",
													Font:        Font{Family: "Segoe UI Symbol", PointSize: 10, Bold: true},
													MaxSize:     Size{Width: 34},
													OnClicked:   refreshDevices,
												},
											}},
											ComboBox{AssignTo: &outputs, Font: Font{Family: "Segoe UI", PointSize: 9}},
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
												PushButton{AssignTo: &startupButton, Text: "Al iniciar", OnClicked: func() {
													b, ok := selected()
													if !ok {
														return
													}
													if b.DeviceID == "cycle" {
														showError(fmt.Errorf("elige una salida concreta para aplicar al iniciar"))
														return
													}
													previous := cfg
													if cfg.Startup != nil && cfg.Startup.DeviceID == b.DeviceID {
														cfg.Startup = nil
														if !persistRules() {
															cfg = previous
															return
														}
														refreshDevices()
														savedNote.SetText("Salida al iniciar eliminada.")
														return
													}
													cfg.Startup = &StartupOutput{DeviceID: b.DeviceID, Name: b.Name}
													if !persistRules() {
														cfg = previous
														return
													}
													refreshDevices()
													savedNote.SetText("Salida al iniciar guardada.")
												}},
												HSpacer{},
												PushButton{AssignTo: &abButton, Text: "Alternar", OnClicked: func() { setABMode(!abEnabled) }},
											}},
											Composite{AssignTo: &abPanel, Visible: false, Layout: HBox{Spacing: 8}, Children: []Widget{
												ComboBox{AssignTo: &outputA, StretchFactor: 1, ToolTipText: "Salida A"},
												ComboBox{AssignTo: &outputB, StretchFactor: 1, ToolTipText: "Salida B"},
											}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{AssignTo: &autoStartButton, Text: "Iniciar con Windows", OnClicked: func() {
													enabled, err := StartupEnabled()
													if err != nil {
														showError(err)
														return
													}
													if err := SetStartupEnabled(!enabled); err != nil {
														showError(err)
														return
													}
													updateAutoStartButton()
												}},
												HSpacer{},
											}},
										},
									},
									Composite{
										Name:          "settings-card",
										StretchFactor: 1,
										Layout:        VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 14}, Spacing: 10},
										Children: []Widget{
											Composite{Layout: HBox{MarginsZero: true, Spacing: 4}, Children: []Widget{
												Label{Text: "Atajos", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
												Label{Text: "(ESC para cancelar)", Font: Font{Family: "Segoe UI", PointSize: 9}},
												HSpacer{},
											}},
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
													if capturedHotkey == "" {
														showError(fmt.Errorf("captura un atajo primero"))
														return
													}
													var b Binding
													if abEnabled {
														first, second, ok := selectedToggle()
														if !ok {
															showError(fmt.Errorf("elige dos salidas distintas para A/B"))
															return
														}
														b = Binding{
															Hotkey:   capturedHotkey,
															DeviceID: "toggle",
															Name:     "Alternar " + first.Name + " / " + second.Name,
															Toggle:   []ToggleTarget{first, second},
														}
													} else {
														var ok bool
														b, ok = selected()
														if !ok {
															return
														}
														b.Hotkey = capturedHotkey
													}
													previous := cfg
													cfg.Bindings = append(append([]Binding{}, cfg.Bindings...), b)
													if !persistRules() {
														cfg = previous
														refreshRules()
														return
													}
													refreshRules()
													capturedHotkey = ""
													capture.stop()
													hotkey.SetText("")
													if abEnabled {
														setABMode(false)
														savedNote.SetText("Atajo A/B guardado.")
													}
												}},
											}},
											Label{Text: "Lista de atajos configurados:", Font: Font{Family: "Segoe UI", PointSize: 9}},
											ListBox{AssignTo: &shortcuts, StretchFactor: 1},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Eliminar", OnClicked: func() {
													i, ok := selectedShortcut()
													if !ok {
														return
													}
													previous := cfg
													cfg.Bindings = append(append([]Binding{}, cfg.Bindings[:i]...), cfg.Bindings[i+1:]...)
													if !persistRules() {
														cfg = previous
														refreshRules()
														return
													}
													refreshRules()
													shortcuts.SetCurrentIndex(-1)
													capturedHotkey = ""
													capture.stop()
													hotkey.SetText("")
													savedNote.SetText("Atajo eliminado.")
												}},
												HSpacer{},
												Label{AssignTo: &savedNote, Text: "", Font: Font{Family: "Segoe UI", PointSize: 8}, TextColor: darkMuted},
											}},
										},
									},
								},
							},
							{
								Title:  "Deej",
								Layout: VBox{Margins: Margins{Left: 18, Top: 16, Right: 18, Bottom: 18}, Spacing: 12},
								Children: []Widget{
									Composite{
										Name:   "settings-card",
										Layout: VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 14}, Spacing: 10},
										Children: []Widget{
											Label{Text: "Conexión con Arduino", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
											Label{Text: "Elige el puerto detectado y ajusta la velocidad de conexión y el suavizado de los sliders.", Font: Font{Family: "Segoe UI", PointSize: 9}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												Label{Text: "Puerto detectado:"},
												ComboBox{AssignTo: &portsCombo, Font: Font{Family: "Segoe UI", PointSize: 11}, MinSize: Size{Width: 320, Height: 32}, MaxSize: Size{Width: 420}},
												PushButton{
													Text:        "⟳",
													ToolTipText: "Volver a detectar puertos",
													Font:        Font{Family: "Segoe UI Symbol", PointSize: 10, Bold: true},
													MaxSize:     Size{Width: 34},
													OnClicked:   refreshPorts,
												},
												HSpacer{},
											}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												Label{Text: "Velocidad de conexión:", ToolTipText: "Baud rate: debe coincidir con el Arduino (normalmente 9600)."},
												NumberEdit{AssignTo: &baudRate, Value: float64(initialBaud), MinValue: 300, MaxValue: 2000000, Decimals: 0, Increment: 100, Font: Font{Family: "Segoe UI", PointSize: 11}, MinSize: Size{Width: 180, Height: 32}, MaxSize: Size{Width: 220}},
												HSpacer{},
												Label{Text: "Suavizado:", ToolTipText: "Bajo responde antes a cambios pequeños; Alto filtra más el temblor del potenciómetro."},
												ComboBox{AssignTo: &noiseCombo, Model: []string{"Bajo", "Normal", "Alto"}, CurrentIndex: initialNoiseIndex, Font: Font{Family: "Segoe UI", PointSize: 11}, MinSize: Size{Width: 180, Height: 32}, MaxSize: Size{Width: 220}},
												HSpacer{},
											}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Guardar conexión", OnClicked: saveDeejConnection},
												HSpacer{},
												Label{AssignTo: &connectionNote, Text: "", Font: Font{Family: "Segoe UI", PointSize: 8}, TextColor: darkMuted},
											}},
										},
									},
									Composite{
										Name:          "settings-card",
										StretchFactor: 1,
										Layout:        VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 14}, Spacing: 10},
										Children: []Widget{
											Label{Text: "Mapeo de sliders", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
											Label{Text: "La lista muestra apps con audio activo; escribe otros nombres o varios separados por comas (discord.exe, chrome.exe).", Font: Font{Family: "Segoe UI", PointSize: 9}},
											ListBox{AssignTo: &slidersList, StretchFactor: 1},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												Label{Text: "Índice:"},
												LineEdit{AssignTo: &sliderIndex, MaxSize: Size{Width: 70}},
												HSpacer{Size: 48},
												Label{Text: "Destino(s):"},
												ComboBox{AssignTo: &sliderTarget, Editable: true, StretchFactor: 1},
												PushButton{
													Text:        "⟳",
													ToolTipText: "Actualizar procesos con audio",
													Font:        Font{Family: "Segoe UI Symbol", PointSize: 10, Bold: true},
													MaxSize:     Size{Width: 34},
													OnClicked:   refreshSliderTargets,
												},
												HSpacer{},
											}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Aplicar a la lista", ToolTipText: "Añade o actualiza el índice en la lista de arriba. Después pulsa Guardar en config.yaml.", OnClicked: applySliderEntry},
												PushButton{Text: "Quitar", OnClicked: removeSliderEntry},
												HSpacer{},
												PushButton{Text: "Recargar del archivo", OnClicked: func() {
													if walk.MsgBox(mw, "Recargar", "¿Descartar los cambios de sliders y leer config.yaml?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
														reloadDeejFile()
													}
												}},
											}},
											Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
												PushButton{Text: "Guardar en config.yaml", OnClicked: saveVisualMapping},
												HSpacer{},
												Label{AssignTo: &visualNote, Text: "", Font: Font{Family: "Segoe UI", PointSize: 8}, TextColor: darkMuted},
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
	mw.SizeChanged().Attach(func() {
		updateShortcutScrollbar(shortcuts)
		updateShortcutScrollbar(slidersList)
	})
	shortcuts.SizeChanged().Attach(func() { updateShortcutScrollbar(shortcuts) })
	slidersList.SizeChanged().Attach(func() { updateShortcutScrollbar(slidersList) })
	outputs.CurrentIndexChanged().Attach(updateStartupButton)
	refreshDevices()
	refreshRules()
	refreshConnectionFields()
	loadVisualMapping()
	updateAutoStartButton()
	shortcuts.CurrentIndexChanged().Attach(func() {
		i, ok := selectedShortcut()
		if !ok {
			return
		}
		b := cfg.Bindings[i]
		capturedHotkey = ""
		capture.stop()
		hotkey.SetText(b.Hotkey)
		loadedOutputIndex = outputIndexForDevice(b.DeviceID)
		outputs.SetCurrentIndex(loadedOutputIndex)
		if strings.EqualFold(b.DeviceID, "toggle") && len(b.Toggle) == 2 {
			setABMode(true)
			if idx := abIndexForDevice(b.Toggle[0].DeviceID); idx >= 0 {
				outputA.SetCurrentIndex(idx)
			}
			if idx := abIndexForDevice(b.Toggle[1].DeviceID); idx >= 0 {
				outputB.SetCurrentIndex(idx)
			}
		} else {
			setABMode(false)
		}
		updateStartupButton()
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

func updateShortcutScrollbar(lb *walk.ListBox) {
	if lb == nil {
		return
	}
	hwnd := lb.Handle()
	style := win.GetWindowLong(hwnd, win.GWL_STYLE)
	style &^= int32(win.WS_HSCROLL)
	count := int(win.SendMessage(hwnd, win.LB_GETCOUNT, 0, 0))
	var client win.RECT
	needed := false
	if win.GetClientRect(hwnd, &client) && count > 0 {
		itemHeight := int(win.SendMessage(hwnd, win.LB_GETITEMHEIGHT, 0, 0))
		needed = itemHeight > 0 && count*itemHeight > int(client.Bottom-client.Top)
	}
	if needed {
		style |= int32(win.WS_VSCROLL)
	} else {
		style &^= int32(win.WS_VSCROLL)
	}
	win.SetWindowLong(hwnd, win.GWL_STYLE, style)
	win.SetWindowPos(hwnd, 0, 0, 0, 0, 0, win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER)
	var show uintptr
	if needed {
		show = 1
	}
	showScrollBar.Call(uintptr(hwnd), uintptr(win.SB_VERT), show)
}
