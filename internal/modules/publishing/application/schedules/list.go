package schedules

import (
	"context"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) List(ctx context.Context, input ListInput) (ports.SchedulePage, error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" || input.Limit < 1 || input.Limit > 100 {
		return ports.SchedulePage{}, ErrInvalidScheduleCommand
	}
	return service.dependencies.Repository.List(ctx, ports.ScheduleListQuery{AccountID: input.AccountID, Limit: input.Limit})
}
