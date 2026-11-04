package deej

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/omriharel/deej/pkg/deej/util"
	"go.uber.org/zap"
)

type sessionMap struct {
	deej   *Deej
	logger *zap.SugaredLogger

	m    map[string][]Session
	lock sync.RWMutex

	sessionFinder SessionFinder

	refreshLock           sync.Mutex
	refreshRequests       chan struct{}
	refreshForceRequested atomic.Bool

	lastSessionRefresh time.Time
	unmappedKeys       []string
}

const (
	masterSessionName = "master" // master device volume
	systemSessionName = "system" // system sounds volume
	inputSessionName  = "mic"    // microphone input level

	// some targets need to be transformed before their correct audio sessions can be accessed.
	// this prefix identifies those targets to ensure they don't contradict with another similarly-named process
	specialTargetTransformPrefix = "deej."

	// targets the currently active window (Windows-only, experimental)
	specialTargetCurrentWindow = "current"

	// targets all currently unmapped sessions (experimental)
	specialTargetAllUnmapped = "unmapped"

	// this threshold constant assumes that re-acquiring all sessions is a kind of expensive operation,
	// and needs to be limited in some manner. this value was previously user-configurable through a config
	// key "process_refresh_frequency", but exposing this type of implementation detail seems wrong now
	minTimeBetweenSessionRefreshes = time.Second * 5

	// determines whether the map should be refreshed when a slider moves.
	// this is a bit greedy but allows us to ensure sessions are always re-acquired, which is
	// especially important for process groups (because you can have one ongoing session
	// always preventing lookup of other processes bound to its slider, which forces the user
	// to manually refresh sessions). a cleaner way to do this down the line is by registering to notifications
	// whenever a new session is added, but that's too hard to justify for how easy this solution is
	maxTimeBetweenSessionRefreshes = time.Second * 45

	// unmapped sessions change more often than the rest of the map (e.g. a game starting),
	// so refresh them a bit more aggressively while a slider actually uses deej.unmapped
	maxTimeBetweenUnmappedRefreshes = time.Second * 15
)

// this matches friendly device names (on Windows), e.g. "Headphones (Realtek Audio)"
var deviceSessionKeyPattern = regexp.MustCompile(`^.+ \(.+\)$`)

func newSessionMap(deej *Deej, logger *zap.SugaredLogger, sessionFinder SessionFinder) (*sessionMap, error) {
	logger = logger.Named("sessions")

	m := &sessionMap{
		deej:            deej,
		logger:          logger,
		m:               make(map[string][]Session),
		sessionFinder:   sessionFinder,
		refreshRequests: make(chan struct{}, 1),
	}

	logger.Debug("Created session map instance")

	return m, nil
}

func (m *sessionMap) initialize() error {
	if err := m.getAndAddSessions(); err != nil {
		m.logger.Warnw("Failed to get all sessions during session map initialization", "error", err)
		return fmt.Errorf("get all sessions during init: %w", err)
	}

	go m.runRefreshWorker()

	m.setupOnConfigReload()
	m.setupOnSliderMove()

	return nil
}

func (m *sessionMap) release() error {
	if err := m.sessionFinder.Release(); err != nil {
		m.logger.Warnw("Failed to release session finder during session map release", "error", err)
		return fmt.Errorf("release session finder during release: %w", err)
	}

	return nil
}

func (m *sessionMap) getAndAddSessions() error {
	newMap, unmappedKeys, err := m.loadSessions()
	if err != nil {
		return err
	}

	m.replaceSessions(newMap, unmappedKeys, time.Now())
	m.logger.Infow("Got all audio sessions successfully", "sessionMap", m)

	return nil
}

func (m *sessionMap) loadSessions() (map[string][]Session, []string, error) {
	sessions, err := m.sessionFinder.GetAllSessions()
	if err != nil {
		m.logger.Warnw("Failed to get sessions from session finder", "error", err)
		return nil, nil, fmt.Errorf("get sessions from SessionFinder: %w", err)
	}

	mapping := m.deej.config.GetSliderMapping()
	dynamicallyMapped := m.currentWindowKeysIfNeeded(mapping)

	newMap := make(map[string][]Session, len(sessions))
	unmappedKeysSet := make(map[string]struct{})

	for _, session := range sessions {
		key := session.Key()

		newMap[key] = append(newMap[key], session)

		if !m.sessionMapped(key, dynamicallyMapped, mapping) {
			m.logger.Debugw("Tracking unmapped session", "session", session)
			unmappedKeysSet[key] = struct{}{}
		}
	}

	unmappedKeys := make([]string, 0, len(unmappedKeysSet))
	for key := range unmappedKeysSet {
		unmappedKeys = append(unmappedKeys, key)
	}
	sort.Strings(unmappedKeys)

	return newMap, unmappedKeys, nil
}

func (m *sessionMap) replaceSessions(newMap map[string][]Session, unmappedKeys []string, refreshedAt time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()

	oldMap := m.m
	m.m = newMap
	m.unmappedKeys = unmappedKeys
	m.lastSessionRefresh = refreshedAt

	// Release the old sessions while holding the write lock. Readers hold the
	// read lock while using a session, so no one can be using these anymore.
	for _, sessions := range oldMap {
		for _, session := range sessions {
			session.Release()
		}
	}
}

func (m *sessionMap) runRefreshWorker() {
	for range m.refreshRequests {
		force := m.refreshForceRequested.Swap(false)
		m.refreshSessions(force)
	}
}

// requestRefresh schedules a refresh without blocking the caller. Multiple
// requests are collapsed into a single pending refresh.
func (m *sessionMap) requestRefresh(force bool) {
	if force {
		m.refreshForceRequested.Store(true)
	}

	select {
	case m.refreshRequests <- struct{}{}:
	default:
		// A refresh is already pending.
	}
}

func (m *sessionMap) setupOnConfigReload() {
	configReloadedChannel := m.deej.config.SubscribeToChanges()

	go func() {
		for range configReloadedChannel {
			m.logger.Info("Detected config reload, attempting to re-acquire all audio sessions")
			m.requestRefresh(false)
		}
	}()
}

func (m *sessionMap) setupOnSliderMove() {
	sliderEventsChannel := m.deej.serial.SubscribeToSliderMoveEvents()

	go func() {
		for event := range sliderEventsChannel {
			m.handleSliderMoveEvent(event)
		}
	}()
}

// performance: explain why force == true at every such use to avoid unintended forced refresh spams
func (m *sessionMap) refreshSessions(force bool) {
	m.refreshLock.Lock()
	defer m.refreshLock.Unlock()

	// make sure enough time passed since the last refresh, unless force is true in which case always clear
	if !force && !m.refreshOlderThan(minTimeBetweenSessionRefreshes) {
		return
	}

	newMap, unmappedKeys, err := m.loadSessions()
	if err != nil {
		m.logger.Warnw("Failed to re-acquire all audio sessions", "error", err)

		// Throttle retries even when the refresh failed.
		m.lock.Lock()
		m.lastSessionRefresh = time.Now()
		m.lock.Unlock()
		return
	}

	m.replaceSessions(newMap, unmappedKeys, time.Now())
	m.logger.Debug("Re-acquired sessions successfully")
}

func (m *sessionMap) refreshOlderThan(maxAge time.Duration) bool {
	m.lock.RLock()
	lastRefresh := m.lastSessionRefresh
	m.lock.RUnlock()

	return lastRefresh.Add(maxAge).Before(time.Now())
}

// returns true if a session is not currently mapped to any slider, false otherwise
// special sessions (master, system, mic) and device-specific sessions always count as mapped,
// even when absent from the config. this makes sense for every current feature that uses "unmapped sessions"
func (m *sessionMap) sessionMapped(key string, dynamicallyMapped map[string]struct{}, mapping *sliderMap) bool {

	// count master/system/mic as mapped
	if isSpecialSessionKey(key) {
		return true
	}

	// count device sessions as mapped
	if deviceSessionKeyPattern.MatchString(key) {
		return true
	}

	// count the currently active window as mapped if deej.current is in use,
	// otherwise an app could be controlled by both deej.current and deej.unmapped
	if _, ok := dynamicallyMapped[key]; ok {
		return true
	}

	if mapping == nil {
		return false
	}

	matchFound := false

	// look through the actual mappings
	mapping.iterate(func(sliderIdx int, targets []string) {
		if matchFound {
			return
		}

		for _, target := range targets {

			// ignore special transforms
			if m.targetHasSpecialTransform(target) {
				continue
			}

			if strings.ToLower(target) == key {
				matchFound = true
				return
			}
		}
	})

	return matchFound
}

func (m *sessionMap) handleSliderMoveEvent(event SliderMoveEvent) {
	mapping := m.deej.config.GetSliderMapping()
	if mapping == nil {
		return
	}

	// first of all, ensure our session map isn't moldy
	if m.refreshOlderThan(maxTimeBetweenSessionRefreshes) {
		m.logger.Debug("Stale session map detected on slider move, refreshing")
		m.requestRefresh(true)
	} else if m.refreshOlderThan(maxTimeBetweenUnmappedRefreshes) &&
		m.configUsesSpecialTarget(mapping, specialTargetAllUnmapped) {
		m.requestRefresh(false)
	}

	// get the targets mapped to this slider from the config
	targets, ok := mapping.get(event.SliderID)

	// if slider not found in config, silently ignore
	if !ok {
		return
	}

	targetFound := false
	adjustmentFailed := false

	// for each possible target for this slider...
	for _, target := range targets {

		// resolve the target name by cleaning it up and applying any special transformations.
		// depending on the transformation applied, this can result in more than one target name
		resolvedTargets := m.resolveTarget(target)

		// for each resolved target...
		for _, resolvedTarget := range resolvedTargets {

			// check the map for matching sessions and adjust them atomically
			found, failed := m.adjustSessions(resolvedTarget, event.PercentValue)
			targetFound = targetFound || found
			adjustmentFailed = adjustmentFailed || failed
		}
	}

	// if we still haven't found a target or the volume adjustment failed, maybe look for the target again.
	// processes could've opened since the last time this slider moved.
	// if they haven't, the cooldown will take care to not spam it up
	if !targetFound {
		m.requestRefresh(false)
	} else if adjustmentFailed {
		m.requestRefresh(true)
	}
}

// adjustSessions keeps the read lock while talking to the sessions. This
// prevents a concurrent refresh from releasing a session that is in use.
func (m *sessionMap) adjustSessions(key string, value float32) (found bool, failed bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	sessions, ok := m.m[key]
	if !ok {
		return false, false
	}

	for _, session := range sessions {
		if session.GetVolume() != value {
			if err := session.SetVolume(value); err != nil {
				m.logger.Warnw("Failed to set target session volume", "error", err)
				failed = true
			}
		}
	}

	return true, failed
}

func (m *sessionMap) targetHasSpecialTransform(target string) bool {
	return strings.HasPrefix(strings.ToLower(target), specialTargetTransformPrefix)
}

func (m *sessionMap) resolveTarget(target string) []string {

	// start by ignoring the case
	target = strings.ToLower(target)

	// look for any special targets first, by examining the prefix
	if m.targetHasSpecialTransform(target) {
		return m.applyTargetTransform(strings.TrimPrefix(target, specialTargetTransformPrefix))
	}

	return []string{target}
}

func (m *sessionMap) applyTargetTransform(specialTargetName string) []string {

	// select the transformation based on its name
	switch specialTargetName {

	// get current active window
	case specialTargetCurrentWindow:
		currentWindowProcessNames, err := util.GetCurrentWindowProcessNames()

		// silently ignore errors here, as this is on deej's "hot path" (and it could just mean the user's running linux)
		if err != nil {
			return nil
		}

		// we could have gotten a non-lowercase names from that, so let's ensure we return ones that are lowercase
		result := make([]string, 0, len(currentWindowProcessNames))
		seen := make(map[string]struct{}, len(currentWindowProcessNames))

		for _, target := range currentWindowProcessNames {
			lowercaseTarget := strings.ToLower(target)
			if _, ok := seen[lowercaseTarget]; ok {
				continue
			}

			seen[lowercaseTarget] = struct{}{}
			result = append(result, lowercaseTarget)
		}

		return result

	// get currently unmapped sessions
	case specialTargetAllUnmapped:
		m.lock.RLock()
		targetKeys := append([]string(nil), m.unmappedKeys...)
		m.lock.RUnlock()

		// If deej.current is in use, exclude the currently active window from
		// the unmapped group to avoid controlling the same app twice.
		if m.configUsesSpecialTarget(m.deej.config.GetSliderMapping(), specialTargetCurrentWindow) {
			if currentWindowProcessNames, err := util.GetCurrentWindowProcessNames(); err == nil {
				excluded := make(map[string]struct{}, len(currentWindowProcessNames))
				for _, target := range currentWindowProcessNames {
					excluded[strings.ToLower(target)] = struct{}{}
				}

				filteredTargetKeys := targetKeys[:0]
				for _, targetKey := range targetKeys {
					if _, ok := excluded[targetKey]; ok {
						continue
					}
					filteredTargetKeys = append(filteredTargetKeys, targetKey)
				}
				targetKeys = filteredTargetKeys
			}
		}

		return targetKeys
	}

	return nil
}

func (m *sessionMap) currentWindowKeysIfNeeded(mapping *sliderMap) map[string]struct{} {
	if !m.configUsesSpecialTarget(mapping, specialTargetCurrentWindow) {
		return nil
	}

	processNames, err := util.GetCurrentWindowProcessNames()
	if err != nil {
		return nil
	}

	result := make(map[string]struct{}, len(processNames))
	for _, processName := range processNames {
		result[strings.ToLower(processName)] = struct{}{}
	}

	return result
}

func (m *sessionMap) configUsesSpecialTarget(mapping *sliderMap, specialTargetName string) bool {
	if mapping == nil {
		return false
	}

	found := false
	targetName := specialTargetTransformPrefix + specialTargetName

	mapping.iterate(func(sliderIdx int, targets []string) {
		if found {
			return
		}

		for _, target := range targets {
			if strings.ToLower(target) == targetName {
				found = true
				return
			}
		}
	})

	return found
}

func isSpecialSessionKey(key string) bool {
	switch key {
	case masterSessionName, systemSessionName, inputSessionName:
		return true
	}

	return false
}

func (m *sessionMap) String() string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	sessionCount := 0

	for _, value := range m.m {
		sessionCount += len(value)
	}

	return fmt.Sprintf("<%d audio sessions>", sessionCount)
}
