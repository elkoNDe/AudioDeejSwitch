package deej

import "go.uber.org/zap"

// Notifier provides generic notification sending
type Notifier interface {
	Notify(title string, message string)
}

// ToastNotifier sends desktop notifications using the platform implementation.
type ToastNotifier struct {
	logger  *zap.SugaredLogger
	desktop desktopNotifier
}

type desktopNotifier interface {
	show(title, message string) error
	close()
}

// NewToastNotifier creates a new ToastNotifier
func NewToastNotifier(logger *zap.SugaredLogger) (*ToastNotifier, error) {
	logger = logger.Named("notifier")
	desktop, err := newDesktopNotifier()
	if err != nil {
		return nil, err
	}
	tn := &ToastNotifier{logger: logger, desktop: desktop}

	logger.Debug("Created toast notifier instance")

	return tn, nil
}

// Notify sends a desktop notification and logs delivery errors.
func (tn *ToastNotifier) Notify(title string, message string) {
	tn.logger.Infow("Sending toast notification", "title", title, "message", message)
	if err := tn.desktop.show(title, message); err != nil {
		tn.logger.Errorw("Failed to send toast notification", "error", err)
	}
}

// Close releases the platform notification host and any temporary tray icon.
func (tn *ToastNotifier) Close() {
	tn.desktop.close()
}
