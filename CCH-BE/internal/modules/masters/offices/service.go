package offices

import (
	"context"
	"database/sql"
	"errors"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type Service struct {
	authorization *authorization.AccessService
	repository    Repository
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{authorization: accessService, repository: repository}
}

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Office, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, code string) (Office, error) {
	if err := service.validateDependencies(); err != nil {
		return Office{}, err
	}
	code, err := NormalizeCode(code)
	if err != nil {
		return Office{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Office{}, err
	}
	value, err := service.repository.Get(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return Office{}, ErrNotFound
	}
	return value, err
}

func (service *Service) LookupOptions(ctx context.Context, identity authentication.VerifiedIdentity) (LookupOptions, error) {
	if err := service.validateDependencies(); err != nil {
		return LookupOptions{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionLookup); err != nil {
		return LookupOptions{}, err
	}
	return service.repository.LookupOptions(ctx)
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Office, error) {
	if err := service.validateDependencies(); err != nil {
		return Office{}, err
	}
	value, err := normalizeOffice(Office{
		Code: command.Code, Name: command.Name, Abbreviation: command.Abbreviation, Type: command.Type,
		RegionalCode: command.RegionalCode, Active: command.Active,
		AsDestination: command.AsDestination, AsCopy: command.AsCopy,
	})
	if err != nil {
		return Office{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Office{}, err
	}
	if err := service.repository.Insert(ctx, value); err != nil {
		return Office{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Office, error) {
	if err := service.validateDependencies(); err != nil {
		return Office{}, err
	}
	value, err := normalizeOffice(Office{
		Code: command.Code, Name: command.Name, Abbreviation: command.Abbreviation, Type: command.Type,
		RegionalCode: command.RegionalCode, Active: command.Active,
		AsDestination: command.AsDestination, AsCopy: command.AsCopy,
	})
	if err != nil {
		return Office{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Office{}, err
	}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Office{}, ErrNotFound
		}
		return Office{}, err
	}
	return value, nil
}

func (service *Service) Deactivate(ctx context.Context, identity authentication.VerifiedIdentity, code string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	code, err := NormalizeCode(code)
	if err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Deactivate(ctx, code)
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}

func normalizeOffice(value Office) (Office, error) {
	var err error
	if value.Code, err = NormalizeCode(value.Code); err != nil {
		return Office{}, err
	}
	if value.Name, err = NormalizeText(value.Name, 50); err != nil {
		return Office{}, err
	}
	if value.Abbreviation, err = NormalizeText(value.Abbreviation, 10); err != nil {
		return Office{}, err
	}
	if value.Type, err = NormalizeText(value.Type, 10); err != nil {
		return Office{}, err
	}
	if value.RegionalCode, err = NormalizeRegionalCode(value.RegionalCode); err != nil {
		return Office{}, err
	}
	return value, nil
}
