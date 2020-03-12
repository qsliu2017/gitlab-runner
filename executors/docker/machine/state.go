package machine

type state int

const (
	machineStateIdle state = iota
	machineStateAcquired
	machineStateCreating
	machineStateUsed
	machineStateRemoving
)

func (t state) String() string {
	switch t {
	case machineStateIdle:
		return "Idle"
	case machineStateAcquired:
		return "Acquired"
	case machineStateCreating:
		return "Creating"
	case machineStateUsed:
		return "Used"
	case machineStateRemoving:
		return "Removing"
	default:
		return "Unknown"
	}
}

func (t state) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}
