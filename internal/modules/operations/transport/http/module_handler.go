package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidModuleHandler = errors.New(
	"operations: invalid module handler",
)

type moduleProjectionReader interface {
	CurrentSnapshot(context.Context) (ports.ReconcileSnapshot, error)
}

type ModuleHandler struct {
	projection moduleProjectionReader
}

func NewModuleHandler(
	projection moduleProjectionReader,
) (*ModuleHandler, error) {
	if projection == nil {
		return nil, ErrInvalidModuleHandler
	}
	return &ModuleHandler{projection: projection}, nil
}

func (handler *ModuleHandler) List(ctx *gin.Context) {
	snapshot, err := handler.projection.CurrentSnapshot(ctx.Request.Context())
	if err != nil {
		writeModuleProblem(ctx)
		return
	}
	items := make([]adminapi.ModuleState, 0, len(snapshot.Modules))
	for _, module := range snapshot.Modules {
		if module.ID == "" ||
			module.ReconcileRevision < 1 ||
			module.ReconcileRevision > int64(^uint(0)>>1) {
			writeModuleProblem(ctx)
			return
		}
		items = append(items, adminapi.ModuleState{
			Id:                module.ID,
			Enabled:           module.Enabled,
			ReconcileRevision: int(module.ReconcileRevision),
		})
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.ModuleListResponse{Items: items})
}

func writeModuleProblem(ctx *gin.Context) {
	response.WriteProblem(
		ctx,
		nethttp.StatusServiceUnavailable,
		middleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			"module_projection_unavailable",
			"The module projection is unavailable.",
			nil,
			nil,
		),
	)
}
