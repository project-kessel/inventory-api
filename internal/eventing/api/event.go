package api

type Event struct {
	// TODO: enumerate the predefined event types.
	EventType string

	// TODO: events may be sent for relationships as well as resource types.
	ResourceType string
	Object       interface{}
}
