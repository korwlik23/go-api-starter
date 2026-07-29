package gorm

import (
	"testing"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	"github.com/stretchr/testify/require"
)

func TestValidateLocaleUpdateRequiresEnabledAcyclicFallback(t *testing.T) {
	firstID := databaseid.ID(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
	)
	secondID := databaseid.ID(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
	)
	records := []localeRecord{
		{
			ID: firstID, Tag: "en", Enabled: true, Version: 2,
		},
		{
			ID: secondID, Tag: "th", Enabled: true,
			FallbackLocaleID: &firstID, Version: 3,
		},
	}
	err := validateLocaleUpdate(records, ports.UpdateLocaleCommand{
		ID:              firstID.String(),
		ExpectedVersion: 2,
		Name:            "English",
		Direction:       localizationdomain.DirectionLTR,
		FallbackID:      secondID.String(),
		Enabled:         true,
		UserSelectable:  true,
	})

	require.ErrorIs(t, err, ports.ErrLocaleConflict)
}

func TestValidateLocaleUpdateProtectsDefaultLocaleLifecycle(t *testing.T) {
	id := databaseid.ID(
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
	)
	defaultSlot := int16(1)
	err := validateLocaleUpdate([]localeRecord{{
		ID: id, Tag: "en", Enabled: true, DefaultSlot: &defaultSlot, Version: 2,
	}}, ports.UpdateLocaleCommand{
		ID:              id.String(),
		ExpectedVersion: 2,
		Name:            "English",
		Direction:       localizationdomain.DirectionLTR,
		Enabled:         false,
	})

	require.ErrorIs(t, err, ports.ErrLocaleConflict)
}
