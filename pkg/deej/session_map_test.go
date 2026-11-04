package deej

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeSession struct {
	key      string
	volume   float32
	setErr   error
	released int
}

func (s *fakeSession) GetVolume() float32 {
	return s.volume
}

func (s *fakeSession) SetVolume(volume float32) error {
	if s.setErr != nil {
		return s.setErr
	}

	s.volume = volume
	return nil
}

func (s *fakeSession) Key() string {
	return s.key
}

func (s *fakeSession) Release() {
	s.released++
}

type fakeSessionFinder struct {
	sessions []Session
	err      error
	released bool
}

func (f *fakeSessionFinder) GetAllSessions() ([]Session, error) {
	if f.err != nil {
		return nil, f.err
	}

	return append([]Session(nil), f.sessions...), nil
}

func (f *fakeSessionFinder) Release() error {
	f.released = true
	return nil
}

func newTestSessionMap(t *testing.T, finder SessionFinder, mapping *sliderMap) *sessionMap {
	t.Helper()

	config := &CanonicalConfig{SliderMapping: mapping}
	deej := &Deej{config: config, logger: zap.NewNop().Sugar()}

	sessions, err := newSessionMap(deej, zap.NewNop().Sugar(), finder)
	if err != nil {
		t.Fatalf("newSessionMap() error = %v", err)
	}

	return sessions
}

func TestLoadSessionsTracksUnmappedKeys(t *testing.T) {
	mapping := newSliderMap()
	mapping.set(0, []string{"chrome.exe", specialTargetTransformPrefix + specialTargetAllUnmapped})

	finder := &fakeSessionFinder{sessions: []Session{
		&fakeSession{key: masterSessionName},
		&fakeSession{key: systemSessionName},
		&fakeSession{key: inputSessionName},
		&fakeSession{key: "chrome.exe"},
		&fakeSession{key: "spotify.exe"},
		&fakeSession{key: "Speakers (Realtek Audio)"},
	}}

	sessions := newTestSessionMap(t, finder, mapping)

	newMap, unmappedKeys, err := sessions.loadSessions()
	if err != nil {
		t.Fatalf("loadSessions() error = %v", err)
	}

	if len(newMap) != 6 {
		t.Errorf("loadSessions() map size = %d, want 6", len(newMap))
	}

	if !reflect.DeepEqual(unmappedKeys, []string{"spotify.exe"}) {
		t.Errorf("loadSessions() unmappedKeys = %v, want [spotify.exe]", unmappedKeys)
	}
}

func TestSessionMapped(t *testing.T) {
	mapping := newSliderMap()
	mapping.set(0, []string{"Chrome.exe", specialTargetTransformPrefix + specialTargetAllUnmapped})

	sessions := newTestSessionMap(t, &fakeSessionFinder{}, mapping)
	dynamicallyMapped := map[string]struct{}{"game.exe": {}}

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "master is always mapped", key: masterSessionName, want: true},
		{name: "system is always mapped", key: systemSessionName, want: true},
		{name: "mic is always mapped", key: inputSessionName, want: true},
		{name: "device sessions are always mapped", key: "Speakers (Realtek Audio)", want: true},
		{name: "current window is mapped", key: "game.exe", want: true},
		{name: "configured target ignores case", key: "chrome.exe", want: true},
		{name: "special transform does not map a process", key: "spotify.exe", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessions.sessionMapped(tt.key, dynamicallyMapped, mapping); got != tt.want {
				t.Errorf("sessionMapped(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}

	if sessions.sessionMapped("spotify.exe", nil, nil) {
		t.Error("sessionMapped() with nil mapping = true, want false")
	}
}

func TestReplaceSessionsReleasesOldSessions(t *testing.T) {
	old := &fakeSession{key: "old.exe"}
	newSession := &fakeSession{key: "new.exe"}

	sessions := newTestSessionMap(t, &fakeSessionFinder{}, nil)
	sessions.m = map[string][]Session{"old.exe": {old}}

	sessions.replaceSessions(map[string][]Session{"new.exe": {newSession}}, nil, time.Now())

	if old.released != 1 {
		t.Errorf("old session release count = %d, want 1", old.released)
	}

	if newSession.released != 0 {
		t.Errorf("new session release count = %d, want 0", newSession.released)
	}

	if got := sessions.m["new.exe"]; len(got) != 1 || got[0] != newSession {
		t.Errorf("replaceSessions() did not install the new map")
	}
}

func TestAdjustSessions(t *testing.T) {
	session := &fakeSession{key: "chrome.exe", volume: 0.2}

	sessions := newTestSessionMap(t, &fakeSessionFinder{}, nil)
	sessions.m = map[string][]Session{"chrome.exe": {session}}

	found, failed := sessions.adjustSessions("chrome.exe", 0.8)
	if !found || failed {
		t.Fatalf("adjustSessions() = (%v, %v), want (true, false)", found, failed)
	}

	if session.volume != 0.8 {
		t.Errorf("session volume = %v, want 0.8", session.volume)
	}

	found, failed = sessions.adjustSessions("missing.exe", 0.8)
	if found || failed {
		t.Errorf("adjustSessions(missing) = (%v, %v), want (false, false)", found, failed)
	}

	session.setErr = errors.New("boom")
	found, failed = sessions.adjustSessions("chrome.exe", 0.4)
	if !found || !failed {
		t.Errorf("adjustSessions(error) = (%v, %v), want (true, true)", found, failed)
	}
}

func TestApplyTargetTransformUnmapped(t *testing.T) {
	sessions := newTestSessionMap(t, &fakeSessionFinder{}, nil)
	sessions.unmappedKeys = []string{"game.exe", "spotify.exe"}

	targets := sessions.resolveTarget("DeeJ.UnMapped")
	if !reflect.DeepEqual(targets, []string{"game.exe", "spotify.exe"}) {
		t.Errorf("resolveTarget(deej.unmapped) = %v, want [game.exe spotify.exe]", targets)
	}

	targets[0] = "changed.exe"
	if sessions.unmappedKeys[0] != "game.exe" {
		t.Error("resolveTarget(deej.unmapped) did not return a copy")
	}

	if got := sessions.resolveTarget("deej.unknown"); got != nil {
		t.Errorf("resolveTarget(deej.unknown) = %v, want nil", got)
	}
}

func TestConfigUsesSpecialTarget(t *testing.T) {
	mapping := newSliderMap()
	mapping.set(0, []string{"chrome.exe", specialTargetTransformPrefix + specialTargetCurrentWindow})

	sessions := newTestSessionMap(t, &fakeSessionFinder{}, mapping)

	if !sessions.configUsesSpecialTarget(mapping, specialTargetCurrentWindow) {
		t.Error("configUsesSpecialTarget(current) = false, want true")
	}

	if sessions.configUsesSpecialTarget(mapping, specialTargetAllUnmapped) {
		t.Error("configUsesSpecialTarget(unmapped) = true, want false")
	}

	if sessions.configUsesSpecialTarget(nil, specialTargetCurrentWindow) {
		t.Error("configUsesSpecialTarget(nil) = true, want false")
	}
}

func TestSessionMapString(t *testing.T) {
	sessions := newTestSessionMap(t, &fakeSessionFinder{}, nil)
	sessions.m = map[string][]Session{
		"chrome.exe":  {&fakeSession{}, &fakeSession{}},
		"spotify.exe": {&fakeSession{}},
	}

	if got := sessions.String(); got != "<3 audio sessions>" {
		t.Errorf("String() = %q, want %q", got, "<3 audio sessions>")
	}
}
