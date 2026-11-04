package deej

import (
	"bufio"
	"errors"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"go.uber.org/zap"

	"github.com/omriharel/deej/pkg/deej/util"
)

var errConnectionCancelled = errors.New("serial: connection cancelled")

// SerialIO provides a deej-aware abstraction layer to managing serial I/O
type SerialIO struct {
	deej   *Deej
	logger *zap.SugaredLogger

	lock sync.Mutex

	connOptions serial.OpenOptions
	conn        io.ReadWriteCloser
	connected   bool

	// controls the read loop and any pending reconnect attempts for the
	// current connection generation
	generation      uint64
	stopChannel     chan struct{}
	shouldReconnect bool

	lastKnownNumSliders        int
	currentSliderPercentValues []float32

	sliderMoveConsumers []chan SliderMoveEvent
}

// SliderMoveEvent represents a single slider move captured by deej
type SliderMoveEvent struct {
	SliderID     int
	PercentValue float32
}

var expectedLinePattern = regexp.MustCompile(`^\d{1,4}(\|\d{1,4})*\r\n$`)

const (
	minReconnectBackoff = time.Second
	maxReconnectBackoff = time.Second * 5
	sliderEventBuffer   = 64
)

// NewSerialIO creates a SerialIO instance that uses the provided deej
// instance's connection info to establish communications with the arduino chip
func NewSerialIO(deej *Deej, logger *zap.SugaredLogger) (*SerialIO, error) {
	logger = logger.Named("serial")

	sio := &SerialIO{
		deej:                deej,
		logger:              logger,
		connected:           false,
		conn:                nil,
		sliderMoveConsumers: []chan SliderMoveEvent{},
	}

	logger.Debug("Created serial i/o instance")

	// respond to config changes
	sio.setupOnConfigReload()

	return sio, nil
}

// Start attempts to connect to our arduino chip. If the port isn't available
// yet, it keeps retrying in the background (except when it is busy).
func (sio *SerialIO) Start() error {
	sio.lock.Lock()

	if sio.connected {
		sio.lock.Unlock()
		sio.logger.Warn("Already connected, can't start another without closing first")
		return errors.New("serial: connection already active")
	}

	// stop any previous generation (read loop or reconnect attempts)
	sio.shouldReconnect = false
	sio.generation++
	gen := sio.generation
	oldStopChannel := sio.stopChannel
	oldConn := sio.conn

	stopChannel := make(chan struct{})
	sio.stopChannel = stopChannel
	sio.conn = nil
	sio.connected = false

	// set minimum read size according to platform (0 for windows, 1 for linux)
	// this prevents a rare bug on windows where serial reads get congested,
	// resulting in significant lag
	minimumReadSize := 0
	if util.Linux() {
		minimumReadSize = 1
	}

	comPort, baudRate := sio.deej.config.GetConnectionInfo()

	sio.connOptions = serial.OpenOptions{
		PortName:        comPort,
		BaudRate:        uint(baudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: uint(minimumReadSize),
	}
	options := sio.connOptions
	sio.shouldReconnect = true
	sio.lock.Unlock()

	if oldStopChannel != nil {
		close(oldStopChannel)
	}
	if oldConn != nil {
		oldConn.Close()
	}

	sio.logger.Debugw("Attempting serial connection",
		"comPort", options.PortName,
		"baudRate", options.BaudRate,
		"minReadSize", minimumReadSize)

	if err := sio.connect(gen, stopChannel, options); err != nil {
		// If the port is busy, retrying forever would just hide the problem.
		// All other errors are treated as "the Arduino isn't ready yet".
		if errors.Is(err, os.ErrPermission) {
			sio.lock.Lock()
			if sio.generation == gen {
				sio.shouldReconnect = false
			}
			sio.lock.Unlock()
			return err
		}

		sio.lock.Lock()
		shouldReconnect := sio.generation == gen && sio.shouldReconnect
		sio.lock.Unlock()

		if shouldReconnect {
			go sio.reconnectLoop(gen, stopChannel)
		}

		return err
	}

	return nil
}

// Stop signals us to shut down our serial connection, if one is active
func (sio *SerialIO) Stop() {
	sio.lock.Lock()
	wasConnected := sio.connected
	stopChannel := sio.stopChannel
	conn := sio.conn

	sio.shouldReconnect = false
	sio.generation++
	sio.stopChannel = nil
	sio.conn = nil
	sio.connected = false
	sio.lock.Unlock()

	if stopChannel != nil {
		close(stopChannel)
	}

	if conn != nil {
		if err := conn.Close(); err != nil {
			sio.logger.Debugw("Failed to close serial connection", "error", err)
		}
	}

	if wasConnected {
		sio.logger.Debug("Shut down serial connection")
	} else {
		sio.logger.Debug("Not currently connected, stopped reconnect attempts")
	}
}

// SubscribeToSliderMoveEvents returns a buffered channel that receives
// a sliderMoveEvent struct every time a slider moves
func (sio *SerialIO) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent, sliderEventBuffer)

	sio.lock.Lock()
	sio.sliderMoveConsumers = append(sio.sliderMoveConsumers, ch)
	sio.lock.Unlock()

	return ch
}

func (sio *SerialIO) setupOnConfigReload() {
	configReloadedChannel := sio.deej.config.SubscribeToChanges()

	const stopDelay = 50 * time.Millisecond

	go func() {
		for range configReloadedChannel {

			// make any config reload unset our slider number to ensure process volumes are being re-set
			// (the next read line will emit SliderMoveEvent instances for all sliders)
			// this needs to happen after a small delay, because the session map will also re-acquire sessions
			// whenever the config file is reloaded, and we don't want it to receive these move events while the map
			// is still cleared. this is kind of ugly, but shouldn't cause any issues
			go func() {
				<-time.After(stopDelay)

				sio.lock.Lock()
				sio.lastKnownNumSliders = 0
				sio.lock.Unlock()
			}()

			comPort, baudRate := sio.deej.config.GetConnectionInfo()

			sio.lock.Lock()
			paramsChanged := comPort != sio.connOptions.PortName ||
				uint(baudRate) != sio.connOptions.BaudRate
			sio.lock.Unlock()

			// if connection params have changed, attempt to stop and start the connection
			if paramsChanged {
				sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
				sio.Stop()

				// let the connection close
				<-time.After(stopDelay)

				if err := sio.Start(); err != nil {
					sio.logger.Warnw("Failed to renew connection after parameter change", "error", err)
				} else {
					sio.logger.Debug("Renewed connection successfully")
				}
			}
		}
	}()
}

func (sio *SerialIO) connect(gen uint64, stopChannel chan struct{}, options serial.OpenOptions) error {
	conn, err := serial.Open(options)
	if err != nil {
		return err
	}

	sio.lock.Lock()
	if sio.generation != gen || !sio.shouldReconnect {
		sio.lock.Unlock()
		conn.Close()
		return errConnectionCancelled
	}

	sio.conn = conn
	sio.connected = true
	// Force the next line to emit events for every slider, even if the
	// Arduino kept sending the same values across a reconnection.
	sio.lastKnownNumSliders = 0
	sio.currentSliderPercentValues = nil
	sio.lock.Unlock()

	namedLogger := sio.logger.Named(strings.ToLower(options.PortName))
	namedLogger.Infow("Connected", "conn", conn)

	go sio.readLoop(gen, stopChannel, conn, namedLogger)

	return nil
}

func (sio *SerialIO) reconnectLoop(gen uint64, stopChannel chan struct{}) {
	backoff := minReconnectBackoff

	for {
		select {
		case <-stopChannel:
			return
		case <-time.After(backoff):
		}

		sio.lock.Lock()
		if sio.generation != gen || !sio.shouldReconnect {
			sio.lock.Unlock()
			return
		}
		options := sio.connOptions
		sio.lock.Unlock()

		err := sio.connect(gen, stopChannel, options)
		if err == nil {
			sio.logger.Info("Reconnected to Arduino")
			sio.deej.notifier.Notify("Arduino reconectado", "La conexión se ha restablecido.")
			return
		}

		sio.logger.Debugw("Serial reconnect attempt failed", "error", err)

		backoff *= 2
		if backoff > maxReconnectBackoff {
			backoff = maxReconnectBackoff
		}
	}
}

func (sio *SerialIO) readLoop(
	gen uint64,
	stopChannel chan struct{},
	conn io.ReadWriteCloser,
	logger *zap.SugaredLogger,
) {
	reader := bufio.NewReader(conn)
	lineChannel := make(chan string)

	go func() {
		defer close(lineChannel)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if sio.deej.Verbose() && !isChannelClosed(stopChannel) {
					logger.Warnw("Failed to read line from serial", "error", err, "line", line)
				}

				return
			}

			if sio.deej.Verbose() {
				logger.Debugw("Read new line", "line", line)
			}

			select {
			case lineChannel <- line:
			case <-stopChannel:
				return
			}
		}
	}()

	for {
		select {
		case <-stopChannel:
			sio.closeConnection(gen, conn, logger)
			return
		case line, ok := <-lineChannel:
			if !ok {
				sio.handleDisconnect(gen, conn, logger)
				return
			}

			sio.handleLine(logger, line)
		}
	}
}

func (sio *SerialIO) handleDisconnect(gen uint64, conn io.ReadWriteCloser, logger *zap.SugaredLogger) {
	sio.lock.Lock()
	if sio.generation != gen || sio.conn != conn {
		sio.lock.Unlock()
		return
	}

	sio.conn = nil
	sio.connected = false
	shouldReconnect := sio.shouldReconnect
	stopChannel := sio.stopChannel
	sio.lock.Unlock()

	if err := conn.Close(); err != nil {
		logger.Debugw("Failed to close disconnected serial connection", "error", err)
	}

	logger.Warn("Serial connection lost")

	if !shouldReconnect || stopChannel == nil {
		return
	}

	// Re-check after closing the connection: Stop() may have run in the
	// meantime, in which case we should not notify or start reconnecting.
	sio.lock.Lock()
	stillActive := sio.generation == gen && sio.shouldReconnect
	sio.lock.Unlock()

	if !stillActive {
		return
	}

	sio.deej.notifier.Notify("Arduino desconectado", "Intentando reconectar...")
	go sio.reconnectLoop(gen, stopChannel)
}

func (sio *SerialIO) closeConnection(gen uint64, conn io.ReadWriteCloser, logger *zap.SugaredLogger) {
	sio.lock.Lock()
	if sio.generation == gen && sio.conn == conn {
		sio.conn = nil
		sio.connected = false
	}
	sio.lock.Unlock()

	if err := conn.Close(); err != nil {
		logger.Debugw("Failed to close serial connection", "error", err)
	}
}

func (sio *SerialIO) handleLine(logger *zap.SugaredLogger, line string) {

	// this function receives an unsanitized line which is guaranteed to end with LF,
	// but most lines will end with CRLF. it may also have garbage instead of
	// deej-formatted values, so we must check for that! just ignore bad ones
	if !expectedLinePattern.MatchString(line) {
		return
	}

	// trim the suffix
	line = strings.TrimSuffix(line, "\r\n")

	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")
	numSliders := len(splitLine)

	invertSliders := sio.deej.config.GetInvertSliders()
	noiseReductionLevel := sio.deej.config.GetNoiseReductionLevel()

	sio.lock.Lock()

	// update our slider count, if needed - this will send slider move events for all
	if numSliders != sio.lastKnownNumSliders {
		logger.Infow("Detected sliders", "amount", numSliders)
		sio.lastKnownNumSliders = numSliders
		sio.currentSliderPercentValues = make([]float32, numSliders)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range sio.currentSliderPercentValues {
			sio.currentSliderPercentValues[idx] = -1.0
		}
	}

	// for each slider:
	moveEvents := []SliderMoveEvent{}
	for sliderIdx, stringValue := range splitLine {

		// convert string values to integers ("1023" -> 1023)
		number, _ := strconv.Atoi(stringValue)

		// turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
		// so let's check the first number for correctness just in case
		if sliderIdx == 0 && number > 1023 {
			sio.lock.Unlock()
			sio.logger.Debugw("Got malformed line from serial, ignoring", "line", line)
			return
		}

		// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
		dirtyFloat := float32(number) / 1023.0

		// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
		normalizedScalar := util.NormalizeScalar(dirtyFloat)

		// if sliders are inverted, take the complement of 1.0
		if invertSliders {
			normalizedScalar = 1 - normalizedScalar
		}

		// check if it changes the desired state (could just be a jumpy raw slider value)
		if util.SignificantlyDifferent(sio.currentSliderPercentValues[sliderIdx], normalizedScalar, noiseReductionLevel) {

			// if it does, update the saved value and create a move event
			sio.currentSliderPercentValues[sliderIdx] = normalizedScalar

			moveEvents = append(moveEvents, SliderMoveEvent{
				SliderID:     sliderIdx,
				PercentValue: normalizedScalar,
			})

			if sio.deej.Verbose() {
				logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
			}
		}
	}

	consumers := append([]chan SliderMoveEvent(nil), sio.sliderMoveConsumers...)
	sio.lock.Unlock()

	// deliver move events if there are any, towards all potential consumers
	for _, consumer := range consumers {
		for _, moveEvent := range moveEvents {
			select {
			case consumer <- moveEvent:
			default:
				// Keep the newest absolute value if the consumer is busy
				// (e.g. while it re-acquires audio sessions).
				select {
				case <-consumer:
				default:
				}

				select {
				case consumer <- moveEvent:
				default:
					sio.logger.Debugw("Dropped slider move event, consumer is busy", "event", moveEvent)
				}
			}
		}
	}
}

func isChannelClosed(channel chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
}
