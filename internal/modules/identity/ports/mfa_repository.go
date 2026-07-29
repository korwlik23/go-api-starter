package ports

import (
	"context"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

type MFAStatusReader interface {
	IsEnabled(ctx context.Context, userID string) (bool, error)
}

type MFAEnrollmentRepository interface {
	SavePending(ctx context.Context, enrollment domain.MFAEnrollment) error
}
