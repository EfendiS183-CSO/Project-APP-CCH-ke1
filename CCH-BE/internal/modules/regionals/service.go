package regionals

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity, query ListQuery) (Page, error) {
	if err := service.validateDependencies(); err != nil {
		return Page{}, err
	}
	query, err := query.Normalize()
	if err != nil {
		return Page{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return Page{}, err
	}
	return service.repository.List(ctx, query)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, code string) (Office, error) {
	if err := service.validateDependencies(); err != nil {
		return Office{}, err
	}
	code, err := normalizeCode(code)
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

func (service *Service) LookupOptions(ctx context.Context, identity authentication.VerifiedIdentity) ([]OfficeOption, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, OfficeTableVariable, authorization.ActionLookup); err != nil {
		return nil, err
	}
	return service.repository.LookupOptions(ctx)
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Office, error) {
	if err := service.validateDependencies(); err != nil {
		return Office{}, err
	}
	value, err := normalizeOffice(Office{
		Code: command.Code, Name: command.Name, Abbreviation: command.Abbreviation,
		Type: command.Type, RegionalCode: command.RegionalCode, Active: command.Active,
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

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}

func normalizeOffice(value Office) (Office, error) {
	var err error
	if value.Code, err = normalizeCode(value.Code); err != nil {
		return Office{}, err
	}
	if value.Name, err = normalizeText(value.Name, 50); err != nil {
		return Office{}, err
	}
	if value.Abbreviation, err = normalizeText(value.Abbreviation, 10); err != nil {
		return Office{}, err
	}
	if value.Type, err = normalizeText(value.Type, 10); err != nil {
		return Office{}, err
	}
	if value.RegionalCode, err = normalizeRegionalCode(value.RegionalCode); err != nil {
		return Office{}, err
	}
	return value, nil
}
