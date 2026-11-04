package deej

import (
	"strings"
	"testing"

	"go.uber.org/zap"
)

func newTestSerialIO() (*SerialIO, *CanonicalConfig) {
	config := &CanonicalConfig{}
	deej := &Deej{
		config:  config,
		logger:  zap.NewNop().Sugar(),
		verbose: false,
	}

	serial := &SerialIO{
		deej:   deej,
		logger: zap.NewNop().Sugar(),
	}

	return serial, config
}

func drainSliderEvents(channel chan SliderMoveEvent) []SliderMoveEvent {
	events := []SliderMoveEvent{}

	for {
		select {
		case event := <-channel:
			events = append(events, event)
		default:
			return events
		}
	}
}

func TestHandleLineEmitsEverySliderOnce(t *testing.T) {
	serial, _ := newTestSerialIO()
	eventsChannel := serial.SubscribeToSliderMoveEvents()

	serial.handleLine(serial.logger, "0|1023\r\n")

	events := drainSliderEvents(eventsChannel)
	if len(events) != 2 {
		t.Fatalf("first line emitted %d events, want 2: %v", len(events), events)
	}

	if events[0].SliderID != 0 || events[0].PercentValue != 0 {
		t.Errorf("event 0 = %+v, want slider 0 at 0", events[0])
	}

	if events[1].SliderID != 1 || events[1].PercentValue != 1 {
		t.Errorf("event 1 = %+v, want slider 1 at 1", events[1])
	}

	serial.handleLine(serial.logger, "0|1023\r\n")

	if events := drainSliderEvents(eventsChannel); len(events) != 0 {
		t.Errorf("unchanged line emitted %d events, want 0: %v", len(events), events)
	}

	serial.handleLine(serial.logger, "512|1023\r\n")

	events = drainSliderEvents(eventsChannel)
	if len(events) != 1 || events[0].SliderID != 0 {
		t.Errorf("changed line emitted %v, want a single event for slider 0", events)
	}
}

func TestHandleLineInvertsSliders(t *testing.T) {
	serial, config := newTestSerialIO()
	config.InvertSliders = true

	eventsChannel := serial.SubscribeToSliderMoveEvents()
	serial.handleLine(serial.logger, "0|1023\r\n")

	events := drainSliderEvents(eventsChannel)
	if len(events) != 2 {
		t.Fatalf("emitted %d events, want 2: %v", len(events), events)
	}

	if events[0].PercentValue != 1 || events[1].PercentValue != 0 {
		t.Errorf("inverted events = %v, want [1 0]", events)
	}
}

func TestHandleLineIgnoresMalformedInput(t *testing.T) {
	serial, _ := newTestSerialIO()
	eventsChannel := serial.SubscribeToSliderMoveEvents()

	messages := []string{
		"not a slider line\r\n",
		"4558|925|41\r\n",
	}

	for _, message := range messages {
		serial.handleLine(serial.logger, message)

		if events := drainSliderEvents(eventsChannel); len(events) != 0 {
			t.Errorf("handleLine(%q) emitted %v, want no events", message, events)
		}
	}
}

func TestHandleLineDropsOldestEventsWhenConsumerIsBusy(t *testing.T) {
	serial, _ := newTestSerialIO()
	eventsChannel := serial.SubscribeToSliderMoveEvents()

	values := make([]string, 65)
	for i := range values {
		values[i] = "1023"
	}

	serial.handleLine(serial.logger, strings.Join(values, "|")+"\r\n")

	events := drainSliderEvents(eventsChannel)
	if len(events) != sliderEventBuffer {
		t.Fatalf("buffered %d events, want %d", len(events), sliderEventBuffer)
	}

	if events[0].SliderID != 1 {
		t.Errorf("first buffered slider = %d, want 1 (oldest event should be dropped)", events[0].SliderID)
	}

	if last := events[len(events)-1]; last.SliderID != 64 {
		t.Errorf("last buffered slider = %d, want 64", last.SliderID)
	}
}

func TestStopWithoutConnectionIsIdempotent(t *testing.T) {
	serial, _ := newTestSerialIO()

	serial.Stop()
	serial.Stop()
}

func TestIsChannelClosed(t *testing.T) {
	channel := make(chan struct{})

	if isChannelClosed(channel) {
		t.Error("isChannelClosed(open) = true, want false")
	}

	close(channel)

	if !isChannelClosed(channel) {
		t.Error("isChannelClosed(closed) = false, want true")
	}
}
