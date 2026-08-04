package application

import (
	"context"
	"errors"
	"strings"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidProfileService = errors.New("invalid profile service")

type ProfileService struct {
	repository   ports.ProfileRepository
	transactions database.Transactor
	now          func() time.Time
}

func NewProfileService(repository ports.ProfileRepository, transactions database.Transactor) (*ProfileService, error) {
	if repository == nil {
		return nil, ErrInvalidProfileService
	}
	return &ProfileService{repository: repository, transactions: transactions, now: time.Now}, nil
}
func (service *ProfileService) Get(ctx context.Context, userID string) (identitydomain.Profile, error) {
	if service == nil || ctx == nil || strings.TrimSpace(userID) == "" {
		return identitydomain.Profile{}, ErrInvalidProfileService
	}
	return service.repository.FindProfile(ctx, userID)
}
func (service *ProfileService) Update(ctx context.Context, userID, displayName, timezone, locale string, expectedVersion int64) (identitydomain.Profile, error) {
	if service == nil || ctx == nil || strings.TrimSpace(userID) == "" {
		return identitydomain.Profile{}, ErrInvalidProfileService
	}
	current, err := service.repository.FindProfile(ctx, userID)
	if err != nil {
		return identitydomain.Profile{}, err
	}
	updated, err := current.Update(displayName, timezone, locale, expectedVersion, service.now().UTC())
	if err != nil {
		return identitydomain.Profile{}, err
	}
	operation := func(operationContext context.Context) error {
		return service.repository.SaveProfile(operationContext, updated, expectedVersion)
	}
	if service.transactions != nil {
		err = service.transactions.WithinTransaction(ctx, operation)
	} else {
		err = operation(ctx)
	}
	if err != nil {
		return identitydomain.Profile{}, err
	}
	return updated, nil
}
