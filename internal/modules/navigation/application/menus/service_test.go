package menus

import (
	"context"
	"errors"
	"testing"
	"time"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

func TestM3B4CreateBuildsStableTreeAndAuditsWithoutTargetSecrets(t *testing.T) {
	t.Parallel()

	repository := &menuRepositoryFake{}
	auditor := &menuAuditorFake{}
	service, err := NewService(Dependencies{
		Repository: repository, Auditor: auditor, Transactions: menuTransactionFunc(func(ctx context.Context, op func(context.Context) error) error { return op(ctx) }),
		NewID: menuIDSequence(), Now: func() time.Time { return time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	result, err := service.Create(context.Background(), CreateInput{
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-menu-1",
		LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", Key: "main", Name: "Main", Enabled: true,
		Items: []ItemInput{
			{Label: "Home", TargetKind: navigationdomain.TargetInternalPath, Target: "/home", Position: 0, Enabled: true,
				Children: []ItemInput{{Label: "Docs", TargetKind: navigationdomain.TargetExternalURL, Target: "https://docs.example.test", Position: 0, Enabled: true}}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "main", result.Key)
	require.Len(t, repository.created.Items, 2)
	require.Equal(t, repository.created.Items[0].ID, repository.created.Items[1].ParentID)
	require.Equal(t, "navigation.menu.created", auditor.event.Action)
	require.NotContains(t, auditor.event.Data, "target")
}

func TestM3B4ReorderRequiresCompleteExistingSetAndPropagatesVersionConflict(t *testing.T) {
	t.Parallel()

	repository := &menuRepositoryFake{current: menuFixture()}
	service, err := NewService(Dependencies{Repository: repository, Auditor: &menuAuditorFake{}, NewID: menuIDSequence(), Now: time.Now})
	require.NoError(t, err)
	_, err = service.Reorder(context.Background(), ReorderInput{
		AccountID: repository.current.AccountID, ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-reorder",
		MenuID: repository.current.ID, ExpectedVersion: repository.current.Version,
		Items: []ReorderItem{{ID: repository.current.Items[0].ID, ParentID: "", Position: 0}},
	})
	require.ErrorIs(t, err, ErrInvalidMenuMutation)

	repository.reorderErr = ports.ErrMenuConflict
	_, err = service.Reorder(context.Background(), ReorderInput{
		AccountID: repository.current.AccountID, ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-reorder",
		MenuID: repository.current.ID, ExpectedVersion: repository.current.Version,
		Items: []ReorderItem{{ID: repository.current.Items[0].ID, Position: 1}, {ID: repository.current.Items[1].ID, Position: 0}},
	})
	require.ErrorIs(t, err, ports.ErrMenuConflict)
}

func TestM3B4MutationRollsBackWhenAuditFails(t *testing.T) {
	t.Parallel()

	auditErr := errors.New("audit unavailable")
	repository := &menuRepositoryFake{}
	service, err := NewService(Dependencies{
		Repository: repository, Auditor: &menuAuditorFake{err: auditErr},
		Transactions: menuTransactionFunc(func(ctx context.Context, op func(context.Context) error) error { return op(ctx) }),
		NewID:        menuIDSequence(), Now: time.Now,
	})
	require.NoError(t, err)
	_, err = service.Create(context.Background(), CreateInput{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", ActorUserID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", RequestID: "request-audit", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", Key: "main", Name: "Main"})
	require.ErrorIs(t, err, auditErr)
}

type menuRepositoryFake struct {
	created    ports.CreateMenuCommand
	current    navigationdomain.Menu
	reorderErr error
}

func (repository *menuRepositoryFake) List(context.Context, ports.MenuListQuery) (ports.MenuListPage, error) {
	return ports.MenuListPage{}, nil
}
func (repository *menuRepositoryFake) Get(context.Context, string, string) (navigationdomain.Menu, error) {
	return repository.current, nil
}
func (repository *menuRepositoryFake) FindPublic(context.Context, string, string, string) (navigationdomain.Menu, error) {
	return repository.current, nil
}
func (repository *menuRepositoryFake) Create(_ context.Context, command ports.CreateMenuCommand) (navigationdomain.Menu, error) {
	repository.created = command
	return command.Menu, nil
}
func (repository *menuRepositoryFake) Update(_ context.Context, command ports.UpdateMenuCommand) (navigationdomain.Menu, error) {
	return command.Menu, nil
}
func (repository *menuRepositoryFake) Delete(context.Context, ports.DeleteMenuCommand) error {
	return nil
}
func (repository *menuRepositoryFake) Reorder(_ context.Context, command ports.ReorderMenuCommand) (navigationdomain.Menu, error) {
	if repository.reorderErr != nil {
		return navigationdomain.Menu{}, repository.reorderErr
	}
	return repository.current, nil
}

type menuAuditorFake struct {
	event MenuAuditEvent
	err   error
}

func (auditor *menuAuditorFake) RecordMenuChange(_ context.Context, event MenuAuditEvent) error {
	auditor.event = event
	return auditor.err
}

type menuTransactionFunc func(context.Context, func(context.Context) error) error

func (transaction menuTransactionFunc) WithinTransaction(ctx context.Context, op func(context.Context) error) error {
	return transaction(ctx, op)
}

var _ database.Transactor = menuTransactionFunc(nil)

func menuIDSequence() func() (string, error) {
	values := []string{
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
	}
	index := 0
	return func() (string, error) {
		value := values[index]
		index++
		return value, nil
	}
}

func menuFixture() navigationdomain.Menu {
	root, _ := navigationdomain.NewMenuItem(navigationdomain.MenuItemInput{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", MenuID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", Label: "A", TargetKind: navigationdomain.TargetInternalPath, Target: "/a", Position: 0, Version: 1})
	child, _ := navigationdomain.NewMenuItem(navigationdomain.MenuItemInput{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67584", AccountID: root.AccountID, MenuID: root.MenuID, Label: "B", TargetKind: navigationdomain.TargetInternalPath, Target: "/b", Position: 1, Version: 1})
	return navigationdomain.Menu{ID: root.MenuID, AccountID: root.AccountID, LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", Key: "main", Name: "Main", Version: 1, Items: []navigationdomain.MenuItem{root, child}}
}
