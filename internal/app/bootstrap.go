package app

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/korwlik23/go-api-starter/internal/modular"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrInvalidBootstrap  = errors.New("app: invalid bootstrap")
	ErrBootstrapConflict = errors.New("app: bootstrap state conflict")
	ErrBootstrapFailed   = errors.New("app: bootstrap failed")
)

var bootstrapSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type BootstrapStatus string

const (
	BootstrapAbsent  BootstrapStatus = "absent"
	BootstrapPresent BootstrapStatus = "present"
	BootstrapPartial BootstrapStatus = "partial"
)

type BootstrapState struct {
	Status    BootstrapStatus
	AccountID string
	UserID    string
}

type BootstrapPermission struct {
	ID    string
	Key   string
	Scope modular.PermissionScope
}

type BootstrapRecord struct {
	AccountID                  string
	AccountSlug                string
	UserID                     string
	Email                      string
	PasswordHash               string
	MembershipID               string
	MembershipRoleAssignmentID string
	AccountRoleID              string
	AccountRoleName            string
	SystemRoleID               string
	SystemRoleName             string
	SystemRoleAssignmentID     string
	Permissions                []BootstrapPermission
}

type BootstrapCommand struct {
	Email           string
	Password        string
	AccountSlug     string
	AccountRoleName string
	SystemRoleName  string
	// Optional IDs are used by the local seed runner. Normal bootstrap leaves
	// them empty and keeps the existing runtime-generated ID behavior.
	AccountID                  string
	UserID                     string
	MembershipID               string
	AccountRoleID              string
	SystemRoleID               string
	MembershipRoleAssignmentID string
	SystemRoleAssignmentID     string
}

type BootstrapResult struct {
	Created   bool
	AccountID string
	UserID    string
}

type BootstrapSession interface {
	Inspect(context.Context, string, string) (BootstrapState, error)
	Create(context.Context, BootstrapRecord) error
	Verify(context.Context, BootstrapRecord) error
}

type BootstrapStore interface {
	WithinBootstrap(
		context.Context,
		func(context.Context, BootstrapSession) error,
	) error
}

type bootstrapAuditor interface {
	Execute(
		context.Context,
		auditapp.AppendCommand,
	) (auditdomain.Event, error)
}

type bootstrapPasswordHasher interface {
	Hash(string) (string, error)
}

type BootstrapDependencies struct {
	Store     BootstrapStore
	Auditor   bootstrapAuditor
	Passwords bootstrapPasswordHasher
	Modules   func() (*modular.Registry, error)
	NewID     func() (string, error)
}

type Bootstrap struct {
	dependencies BootstrapDependencies
}

func NewBootstrap(
	dependencies BootstrapDependencies,
) (*Bootstrap, error) {
	if dependencies.Store == nil ||
		dependencies.Auditor == nil ||
		dependencies.Passwords == nil ||
		dependencies.Modules == nil ||
		dependencies.NewID == nil {
		return nil, ErrInvalidBootstrap
	}
	return &Bootstrap{dependencies: dependencies}, nil
}

func (bootstrap *Bootstrap) Execute(
	ctx context.Context,
	command BootstrapCommand,
) (BootstrapResult, error) {
	normalized, err := validateBootstrapCommand(command)
	if bootstrap == nil || ctx == nil || err != nil {
		return BootstrapResult{}, ErrInvalidBootstrap
	}
	registry, err := bootstrap.dependencies.Modules()
	if err != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}
	permissions := bootstrapPermissionCatalog(registry)
	passwordHash, err := bootstrap.dependencies.Passwords.Hash(normalized.Password)
	if err != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}

	result := BootstrapResult{}
	err = bootstrap.dependencies.Store.WithinBootstrap(
		ctx,
		func(transactionContext context.Context, session BootstrapSession) error {
			if session == nil {
				return ErrBootstrapFailed
			}
			state, inspectErr := session.Inspect(
				transactionContext,
				normalized.Email,
				normalized.AccountSlug,
			)
			if inspectErr != nil {
				return inspectErr
			}
			switch state.Status {
			case BootstrapPartial:
				return ErrBootstrapConflict
			case BootstrapPresent:
				record, recordErr := existingBootstrapRecord(
					state,
					normalized,
					permissions,
				)
				if recordErr != nil {
					return recordErr
				}
				if verifyErr := session.Verify(transactionContext, record); verifyErr != nil {
					return verifyErr
				}
				result = BootstrapResult{
					AccountID: state.AccountID,
					UserID:    state.UserID,
				}
				return nil
			case BootstrapAbsent:
				record, recordErr := bootstrap.newRecord(
					normalized,
					passwordHash,
					permissions,
				)
				if recordErr != nil {
					return recordErr
				}
				if createErr := session.Create(transactionContext, record); createErr != nil {
					return createErr
				}
				if _, auditErr := bootstrap.dependencies.Auditor.Execute(
					transactionContext,
					auditapp.AppendCommand{
						Scope:  auditdomain.ScopeSystem,
						Actor:  auditdomain.Actor{Type: auditdomain.ActorSystem},
						Action: "bootstrap.foundation.created",
						Data: map[string]any{
							"account_id":       record.AccountID,
							"user_id":          record.UserID,
							"account_role_id":  record.AccountRoleID,
							"system_role_id":   record.SystemRoleID,
							"permission_count": len(record.Permissions),
						},
					},
				); auditErr != nil {
					return auditErr
				}
				result = BootstrapResult{
					Created:   true,
					AccountID: record.AccountID,
					UserID:    record.UserID,
				}
				return nil
			default:
				return ErrBootstrapConflict
			}
		},
	)
	if err == nil {
		return result, nil
	}
	if errors.Is(err, ErrBootstrapConflict) {
		return BootstrapResult{}, ErrBootstrapConflict
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return BootstrapResult{}, err
	}
	return BootstrapResult{}, ErrBootstrapFailed
}

func (bootstrap *Bootstrap) newRecord(
	command BootstrapCommand,
	passwordHash string,
	permissions []BootstrapPermission,
) (BootstrapRecord, error) {
	accountID, err := optionalOrNewID(command.AccountID, bootstrap.dependencies.NewID)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapFailed
	}
	userID, err := optionalOrNewID(command.UserID, bootstrap.dependencies.NewID)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapFailed
	}
	membershipID, err := optionalOrNewID(command.MembershipID, bootstrap.dependencies.NewID)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapFailed
	}
	membershipRoleAssignmentID, err := optionalOrNewID(
		command.MembershipRoleAssignmentID,
		bootstrap.dependencies.NewID,
	)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapFailed
	}
	systemRoleAssignmentID, err := optionalOrNewID(
		command.SystemRoleAssignmentID,
		bootstrap.dependencies.NewID,
	)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapFailed
	}
	state := BootstrapState{
		Status:    BootstrapPresent,
		AccountID: accountID,
		UserID:    userID,
	}
	record, err := existingBootstrapRecord(state, command, permissions)
	if err != nil {
		return BootstrapRecord{}, err
	}
	record.PasswordHash = passwordHash
	if command.AccountRoleID != "" {
		record.AccountRoleID = command.AccountRoleID
	}
	if command.SystemRoleID != "" {
		record.SystemRoleID = command.SystemRoleID
	}
	record.MembershipID = membershipID
	record.MembershipRoleAssignmentID = membershipRoleAssignmentID
	record.SystemRoleAssignmentID = systemRoleAssignmentID
	return record, nil
}

func existingBootstrapRecord(
	state BootstrapState,
	command BootstrapCommand,
	permissions []BootstrapPermission,
) (BootstrapRecord, error) {
	accountRoleID, err := derivedRoleID(
		state.AccountID,
		"go-api-starter.bootstrap.account-role.v1",
	)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapConflict
	}
	systemRoleID, err := derivedRoleID(
		state.UserID,
		"go-api-starter.bootstrap.system-role.v1",
	)
	if err != nil {
		return BootstrapRecord{}, ErrBootstrapConflict
	}
	if command.AccountRoleID != "" {
		accountRoleID = command.AccountRoleID
	}
	if command.SystemRoleID != "" {
		systemRoleID = command.SystemRoleID
	}
	return BootstrapRecord{
		AccountID:       state.AccountID,
		AccountSlug:     command.AccountSlug,
		UserID:          state.UserID,
		Email:           command.Email,
		AccountRoleID:   accountRoleID,
		AccountRoleName: command.AccountRoleName,
		SystemRoleID:    systemRoleID,
		SystemRoleName:  command.SystemRoleName,
		Permissions:     append([]BootstrapPermission(nil), permissions...),
	}, nil
}

func derivedRoleID(namespace, purpose string) (string, error) {
	namespaceID, err := uuid.Parse(namespace)
	if err != nil {
		return "", err
	}
	return uuid.NewSHA1(namespaceID, []byte(purpose)).String(), nil
}

func bootstrapPermissionCatalog(
	registry *modular.Registry,
) []BootstrapPermission {
	if registry == nil {
		return nil
	}
	permissions := make([]BootstrapPermission, 0)
	for _, manifest := range registry.Ordered() {
		for _, permission := range manifest.Permissions {
			permissions = append(permissions, BootstrapPermission{
				Key:   permission.Key,
				Scope: permission.Scope,
			})
		}
	}
	sort.Slice(permissions, func(left, right int) bool {
		return permissions[left].Key < permissions[right].Key
	})
	return permissions
}

func validateBootstrapCommand(command BootstrapCommand) (BootstrapCommand, error) {
	command.Email = identitydomain.NormalizeEmail(command.Email)
	command.AccountSlug = strings.TrimSpace(command.AccountSlug)
	command.AccountRoleName = strings.TrimSpace(command.AccountRoleName)
	command.SystemRoleName = strings.TrimSpace(command.SystemRoleName)
	address, err := mail.ParseAddress(command.Email)
	if err != nil ||
		address.Address != command.Email ||
		len(command.Password) < 12 ||
		len(command.Password) > 128 ||
		!utf8.ValidString(command.Password) ||
		!bootstrapSlugPattern.MatchString(command.AccountSlug) ||
		!validBootstrapDisplayName(command.AccountRoleName) ||
		!validBootstrapDisplayName(command.SystemRoleName) {
		return BootstrapCommand{}, ErrInvalidBootstrap
	}
	for _, value := range []string{
		command.AccountID,
		command.UserID,
		command.MembershipID,
		command.AccountRoleID,
		command.SystemRoleID,
		command.MembershipRoleAssignmentID,
		command.SystemRoleAssignmentID,
	} {
		if value != "" {
			if _, parseErr := uuid.Parse(value); parseErr != nil {
				return BootstrapCommand{}, ErrInvalidBootstrap
			}
		}
	}
	return command, nil
}

func optionalOrNewID(value string, newID func() (string, error)) (string, error) {
	if value != "" {
		return value, nil
	}
	return newID()
}

func validBootstrapDisplayName(value string) bool {
	return value != "" &&
		len(value) <= 128 &&
		utf8.ValidString(value)
}
