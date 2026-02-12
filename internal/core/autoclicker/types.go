package autoclicker

import "time"

const (
	EventTypeSyn uint16 = 0x00
	EventTypeKey uint16 = 0x01
	EventTypeRel uint16 = 0x02
	EventTypeAbs uint16 = 0x03

	SynReportCode  uint16 = 0
	LeftButtonCode uint16 = 0x110
)

type Event struct {
	Type  uint16
	Code  uint16
	Value int32
}

type Config struct {
	TriggerCode    uint16
	ToggleCode     uint16
	TriggerSources map[string]struct{}
	ToggleSources  map[string]struct{}
	GrabSources    map[string]struct{}
	GrabEnabled    bool
	CPS            float64
	ClickDown      time.Duration
	StartEnabled   bool
}

type Injector interface {
	WriteEvents(events ...Event) error
	Close() error
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
