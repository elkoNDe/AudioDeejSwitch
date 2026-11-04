package deej

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	ole "github.com/go-ole/go-ole"
	ps "github.com/mitchellh/go-ps"
	wca "github.com/moutend/go-wca"
	"go.uber.org/zap"
)

var errNoSuchProcess = errors.New("No such process")
var errRefreshSessions = errors.New("Trigger session refresh")

type wcaSession struct {
	baseSession

	pid         uint32
	processName string

	worker *comWorker

	control *wca.IAudioSessionControl2
	volume  *wca.ISimpleAudioVolume

	eventCtx *ole.GUID
}

type masterSession struct {
	baseSession

	worker *comWorker

	volume *wca.IAudioEndpointVolume

	eventCtx *ole.GUID

	stale atomic.Bool // when set to true, we should refresh sessions on the next call to SetVolume
}

func newWCASession(
	logger *zap.SugaredLogger,
	worker *comWorker,
	control *wca.IAudioSessionControl2,
	volume *wca.ISimpleAudioVolume,
	pid uint32,
	eventCtx *ole.GUID,
) (*wcaSession, error) {

	s := &wcaSession{
		worker:   worker,
		control:  control,
		volume:   volume,
		pid:      pid,
		eventCtx: eventCtx,
	}

	// special treatment for system sounds session
	if pid == 0 {
		s.system = true
		s.name = systemSessionName
		s.humanReadableDesc = "system sounds"
	} else {

		// find our session's process name
		process, err := ps.FindProcess(int(pid))
		if err != nil {
			logger.Warnw("Failed to find process name by ID", "pid", pid, "error", err)

			// We're already on the COM thread, so release the handles directly
			// instead of dispatching back through the worker.
			control.Release()
			volume.Release()

			return nil, fmt.Errorf("find process name by pid: %w", err)
		}

		// this PID may be invalid - this means the process has already been
		// closed and we shouldn't create a session for it. The caller owns the
		// handles in this case and releases them after seeing errNoSuchProcess.
		if process == nil {
			logger.Debugw("Process already exited, not creating audio session", "pid", pid)

			return nil, errNoSuchProcess
		}

		s.processName = process.Executable()
		s.name = s.processName
		s.humanReadableDesc = fmt.Sprintf("%s (pid %d)", s.processName, s.pid)
	}

	// use a self-identifying session name e.g. deej.sessions.chrome.
	// Do not log the session object itself here: String() reads the volume,
	// which would dispatch back into the COM worker and deadlock.
	s.logger = logger.Named(strings.TrimSuffix(s.Key(), ".exe"))
	s.logger.Debugw(sessionCreationLogMessage,
		"session", s.name,
		"pid", s.pid,
		"system", s.system)

	return s, nil
}

func newMasterSession(
	logger *zap.SugaredLogger,
	worker *comWorker,
	volume *wca.IAudioEndpointVolume,
	eventCtx *ole.GUID,
	key string,
	loggerKey string,
) (*masterSession, error) {

	s := &masterSession{
		worker:   worker,
		volume:   volume,
		eventCtx: eventCtx,
	}

	s.logger = logger.Named(loggerKey)
	s.master = true
	s.name = key
	s.humanReadableDesc = key

	s.logger.Debugw(sessionCreationLogMessage, "session", key)

	return s, nil
}

func (s *wcaSession) GetVolume() float32 {
	var level float32

	if err := s.worker.call(func() {
		level = s.getVolumeOnWorker()
	}); err != nil {
		s.logger.Warnw("Failed to get session volume", "error", err)
	}

	return level
}

func (s *wcaSession) getVolumeOnWorker() float32 {
	var level float32

	if err := s.volume.GetMasterVolume(&level); err != nil {
		s.logger.Warnw("Failed to get session volume", "error", err)
	}

	return level
}

func (s *wcaSession) SetVolume(v float32) error {
	var result error

	if err := s.worker.call(func() {
		result = s.setVolumeOnWorker(v)
	}); err != nil {
		return fmt.Errorf("run session volume adjustment: %w", err)
	}

	return result
}

func (s *wcaSession) setVolumeOnWorker(v float32) error {
	if err := s.volume.SetMasterVolume(v, s.eventCtx); err != nil {
		s.logger.Warnw("Failed to set session volume", "error", err)
		return fmt.Errorf("adjust session volume: %w", err)
	}

	// mitigate expired sessions by checking the state whenever we change volumes
	var state uint32

	if err := s.control.GetState(&state); err != nil {
		s.logger.Warnw("Failed to get session state while setting volume", "error", err)
		return fmt.Errorf("get session state: %w", err)
	}

	if state == wca.AudioSessionStateExpired {
		s.logger.Warnw("Audio session expired, triggering session refresh")
		return errRefreshSessions
	}

	s.logger.Debugw("Adjusting session volume", "to", fmt.Sprintf("%.2f", v))

	return nil
}

func (s *wcaSession) Release() {
	if err := s.worker.call(s.releaseOnWorker); err != nil {
		s.logger.Debugw("Failed to release audio session", "error", err)
	}
}

func (s *wcaSession) releaseOnWorker() {
	s.logger.Debug("Releasing audio session")

	if s.volume != nil {
		s.volume.Release()
		s.volume = nil
	}

	if s.control != nil {
		s.control.Release()
		s.control = nil
	}
}

func (s *wcaSession) String() string {
	return fmt.Sprintf(sessionStringFormat, s.humanReadableDesc, s.GetVolume())
}

func (s *masterSession) GetVolume() float32 {
	var level float32

	if err := s.worker.call(func() {
		level = s.getVolumeOnWorker()
	}); err != nil {
		s.logger.Warnw("Failed to get session volume", "error", err)
	}

	return level
}

func (s *masterSession) getVolumeOnWorker() float32 {
	var level float32

	if err := s.volume.GetMasterVolumeLevelScalar(&level); err != nil {
		s.logger.Warnw("Failed to get session volume", "error", err)
	}

	return level
}

func (s *masterSession) SetVolume(v float32) error {
	var result error

	if err := s.worker.call(func() {
		result = s.setVolumeOnWorker(v)
	}); err != nil {
		return fmt.Errorf("run session volume adjustment: %w", err)
	}

	return result
}

func (s *masterSession) setVolumeOnWorker(v float32) error {
	if s.stale.Load() {
		s.logger.Warnw("Session expired because default device has changed, triggering session refresh")
		return errRefreshSessions
	}

	if err := s.volume.SetMasterVolumeLevelScalar(v, s.eventCtx); err != nil {
		s.logger.Warnw("Failed to set session volume",
			"error", err,
			"volume", v)

		return fmt.Errorf("adjust session volume: %w", err)
	}

	s.logger.Debugw("Adjusting session volume", "to", fmt.Sprintf("%.2f", v))

	return nil
}

func (s *masterSession) Release() {
	if err := s.worker.call(s.releaseOnWorker); err != nil {
		s.logger.Debugw("Failed to release audio session", "error", err)
	}
}

func (s *masterSession) releaseOnWorker() {
	s.logger.Debug("Releasing audio session")

	if s.volume != nil {
		s.volume.Release()
		s.volume = nil
	}
}

func (s *masterSession) String() string {
	return fmt.Sprintf(sessionStringFormat, s.humanReadableDesc, s.GetVolume())
}

func (s *masterSession) markAsStale() {
	s.stale.Store(true)
}
