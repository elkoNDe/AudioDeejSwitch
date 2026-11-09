package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/omriharel/deej/internal/audioswitch"
	"github.com/omriharel/deej/internal/updater"

	"github.com/omriharel/deej/pkg/deej"
)

var (
	gitCommit  string
	versionTag string
	buildType  string

	verbose  bool
	settings bool
)

func init() {
	flag.BoolVar(&verbose, "verbose", false, "show verbose logs (useful for debugging serial)")
	flag.BoolVar(&verbose, "v", false, "shorthand for --verbose")
	flag.BoolVar(&settings, "settings", false, "open AudioDeejSwitch settings without connecting to Arduino")
	flag.Parse()
}

func main() {
	// Portable configs belong next to the executable, even when launched by a
	// startup shortcut whose working directory is somewhere else.
	if exe, err := os.Executable(); err == nil {
		if err := os.Chdir(filepath.Dir(exe)); err != nil {
			panic(err)
		}
	}
	if settings {
		if err := audioswitch.RunSettings(); err != nil {
			showSettingsError(err)
		}
		return
	}

	// first we need a logger
	logger, err := deej.NewLogger(buildType)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	named := logger.Named("main")
	named.Debug("Created logger")

	// Only the main process holds the lock. The settings window is spawned as a
	// child process with --settings and must be allowed to run alongside it.
	releaseInstance, err := audioswitch.EnsureSingleInstance("AudioDeejSwitch")
	if err != nil {
		if errors.Is(err, audioswitch.ErrAlreadyRunning) {
			named.Warn("Another AudioDeejSwitch instance is already running, exiting")
			return
		}
		named.Warnw("Failed to acquire single-instance lock", "error", err)
	} else {
		defer releaseInstance()
	}

	named.Infow("Version info",
		"gitCommit", gitCommit,
		"versionTag", versionTag,
		"buildType", buildType)

	// provide a fair warning if the user's running in verbose mode
	if verbose {
		named.Debug("Verbose flag provided, all log messages will be shown")
	}

	// create the deej instance
	d, err := deej.NewDeej(logger, verbose)
	if err != nil {
		named.Fatalw("Failed to create deej object", "error", err)
	}

	// Manual update checks are always available from the tray. Automatic checks
	// only run when switcher.yaml explicitly opts in with check_updates.enabled.
	d.SetUpdateChecks(updater.CheckUpdate)
	if switcherConfig, cfgErr := audioswitch.LoadConfig(); cfgErr == nil &&
		switcherConfig.CheckUpdates != nil && switcherConfig.CheckUpdates.Enabled {
		go func() {
			time.Sleep(3 * time.Second)
			d.CheckForUpdatesAndNotify(context.Background())
		}()
	}

	// if injected by build process, set version info to show up in the tray
	if buildType != "" && (versionTag != "" || gitCommit != "") {
		identifier := gitCommit
		if versionTag != "" {
			identifier = versionTag
		}

		d.SetVersion(identifier)
	}

	// onwards, to glory
	if err = d.Initialize(); err != nil {
		named.Fatalw("Failed to initialize deej", "error", err)
	}
}
