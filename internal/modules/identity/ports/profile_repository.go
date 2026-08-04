package ports

import (
	"context"
	"errors"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrProfileConflict = errors.New("profile conflict")
)

type ProfileRepository interface {
	FindProfile(context.Context, string) (domain.Profile, error)
	SaveProfile(context.Context, domain.Profile, int64) error
}
