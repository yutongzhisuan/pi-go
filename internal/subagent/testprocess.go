package subagent

// NewFixedProcess builds a Process with a predetermined event stream and Wait
// result. Used by workersidecar/backend tests to simulate child pi behavior.
func NewFixedProcess(events []Event, waitResult string, waitErr error) *Process {
	p := &Process{
		events: make(chan Event, len(events)+1),
		done:   make(chan struct{}),
		result: waitResult,
		err:    waitErr,
	}
	for _, ev := range events {
		p.events <- ev
	}
	close(p.events)
	close(p.done)
	return p
}
