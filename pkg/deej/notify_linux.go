package deej

import (
	"os"
	"path/filepath"

	"github.com/gen2brain/beeep"
	"github.com/omriharel/deej/pkg/deej/icon"
	"github.com/omriharel/deej/pkg/deej/util"
)

type linuxNotifier struct{}

func newDesktopNotifier() (desktopNotifier, error) {
	return linuxNotifier{}, nil
}

func (linuxNotifier) close() {}

func (linuxNotifier) show(title, message string) error {
	appIconPath := filepath.Join(os.TempDir(), "deej.ico")
	if !util.FileExists(appIconPath) {
		if err := os.WriteFile(appIconPath, icon.DeejLogo, 0644); err != nil {
			return err
		}
	}
	return beeep.Notify(title, message, appIconPath)
}
