package audioswitch

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

func showHotkeysSaved(owner walk.Form) (bool, error) {
	var dialog *walk.Dialog
	var accept *walk.PushButton
	var hide *walk.CheckBox
	hideChecked := false
	err := (Dialog{
		AssignTo:      &dialog,
		Title:         "Atajos guardados",
		Icon:          owner.Icon(),
		Font:          Font{Family: "Segoe UI", PointSize: 9},
		FixedSize:     true,
		DefaultButton: &accept,
		CancelButton:  &accept,
		Layout:        VBox{Margins: Margins{Left: 22, Top: 18, Right: 22, Bottom: 18}, Spacing: 16},
		Children: []Widget{
			Label{Text: "Los atajos se han guardado correctamente.", Font: Font{Family: "Segoe UI", PointSize: 11, Bold: true}},
			Label{Text: "Se aplican en aproximadamente un segundo si AudioDeejSwitch está en ejecución."},
			Label{Text: "Si un atajo está ocupado, aparecerá una notificación."},
			CheckBox{AssignTo: &hide, Text: "No volver a mostrar este mensaje"},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &accept, Text: "Aceptar", MinSize: Size{Width: 90}, OnClicked: func() {
					hideChecked = hide.Checked()
					dialog.Accept()
				}},
			}},
		},
	}).Create(owner)
	if err != nil {
		return false, err
	}
	defer dialog.Dispose()
	if err := applySettingsDarkMode(dialog); err != nil {
		return false, err
	}
	accepted := dialog.Run() == walk.DlgCmdOK
	return accepted && hideChecked, nil
}
