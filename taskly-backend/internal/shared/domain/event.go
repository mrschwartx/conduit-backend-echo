package domain

import "time"

const UserDeletedEvent = "user.deleted"

type UserDeleted struct {
	UserID string
}

type Event struct {
	Name       string
	Payload    any
	OccurredAt time.Time
}

func NewEvent(name string, payload any, occurredAt time.Time) Event {
	return Event{Name: name, Payload: payload, OccurredAt: occurredAt.UTC()}
}
