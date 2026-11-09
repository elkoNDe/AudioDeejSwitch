package deej

import (
	"context"

	"github.com/getlantern/systray"
	"github.com/omriharel/deej/internal/audioswitch"

	"github.com/omriharel/deej/pkg/deej/icon"
	"github.com/omriharel/deej/pkg/deej/util"
)

func (d *Deej) initializeTray(onDone func()) {
	logger := d.logger.Named("tray")

	onReady := func() {
		logger.Debug("Tray instance ready")

		systray.SetIcon(icon.AppLogo)
		systray.SetTitle("AudioDeejSwitch")
		systray.SetTooltip("AudioDeejSwitch")

		settings := systray.AddMenuItem("Configuración", "Abrir AudioDeejSwitch")
		settings.SetIcon(icon.SettingsRed)
		if util.Linux() {
			settings.Hide()
		}
		systray.AddSeparator()

		checkUpdates := systray.AddMenuItem("Buscar actualizaciones", "Verificar si hay una nueva versión disponible")
		checkUpdates.SetIcon(icon.RefreshSessionsRed)

		if d.version != "" {
			systray.AddSeparator()
			versionInfo := systray.AddMenuItem(d.version, "")
			versionInfo.Disable()
		}

		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Stop deej and quit")

		// wait on things to happen
		go func() {
			for {
				select {
				case <-settings.ClickedCh:
					if err := audioswitch.OpenSettings(); err != nil {
						logger.Warnw("Open settings", "error", err)
					}

					// quit
				case <-quit.ClickedCh:
					logger.Info("Quit menu item clicked, stopping")

					audioswitch.CloseSettings()
					d.signalStop()

				// check updates
				case <-checkUpdates.ClickedCh:
					logger.Info("Check updates menu item clicked")
					go d.CheckForUpdatesAndNotify(context.Background())
				}
			}
		}()

		// actually start the main runtime
		onDone()
	}

	onExit := func() {
		logger.Debug("Tray exited")
	}

	// start the tray icon
	logger.Debug("Running in tray")
	systray.Run(onReady, onExit)
}

func (d *Deej) stopTray() {
	d.logger.Debug("Quitting tray")
	systray.Quit()
}
