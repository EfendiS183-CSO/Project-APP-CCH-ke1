package handlingstatuses

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Status, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, code string) (Status, error) {
	if err := service.validateDependencies(); err != nil {
		return Status{}, err
	}
	code, err := normalizeCode(code)
	if err != nil {
		return Status{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Status{}, err
	}
	value, err := service.repository.Get(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return Status{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Status, error) {
	if err := service.validateDependencies(); err != nil {
		return Status{}, err
	}
	code, err := normalizeCode(command.Code)
	if err != nil {
		return Status{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return Status{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Status{}, err
	}
	value := Status{Code: code, Description: description, Active: command.Active}
	if err := service.repository.Insert(ctx, value); err != nil {
		return Status{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Status, error) {
	if err := service.validateDependencies(); err != nil {
		return Status{}, err
	}
	code, err := normalizeCode(command.Code)
	if err != nil {
		return Status{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return Status{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Status{}, err
	}
	value := Status{Code: code, Description: description, Active: command.Active}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Status{}, ErrNotFound
		}
		return Status{}, err
	}
	return value, nil
}

func (service *Service) Deactivate(ctx context.Context, identity authentication.VerifiedIdentity, code string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	code, err := normalizeCode(code)
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
