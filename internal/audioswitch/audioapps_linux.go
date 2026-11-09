package audioswitch

// RunningAudioApps returns a list of executable names (lowercase) that currently
// have an active audio render session on Linux.
// This is a stub implementation; on Linux we rely on PulseAudio/PipeWire which
// is not wired up yet. Return empty slice.
func RunningAudioApps() []string {
	return nil
}
