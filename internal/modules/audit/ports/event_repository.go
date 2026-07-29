package ports

import (
	"context"
	"time"

	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
)

type Cursor struct {
	RecordedAt time.Time
	ID         string
}

type EventPage struct {
	Items      []auditdomain.Event
	NextCursor *Cursor
}

type EventRepository interface {
	Append(context.Context, auditdomain.Event) error
	ListAccount(
		context.Context,
		string,
		*Cursor,
		int,
	) (EventPage, error)
	ListSystem(
		context.Context,
		*Cursor,
		int,
	) (EventPage, error)
}
