package helpcategories

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Category, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, id int64) (Category, error) {
	if err := service.validateDependencies(); err != nil {
		return Category{}, err
	}
	if err := validateID(id); err != nil {
		return Category{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Category{}, err
	}
	value, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Languages(ctx context.Context, identity authentication.VerifiedIdentity) ([]Language, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionLookup); err != nil {
		return nil, err
	}
	return service.repository.Languages(ctx)
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Category, error) {
	if err := service.validateDependencies(); err != nil {
		return Category{}, err
	}
	language, description, err := normalizeCategory(command.Language, command.Description)
	if err != nil {
		return Category{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Category{}, err
	}
	return service.repository.Insert(ctx, Category{Language: language, Description: description})
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Category, error) {
	if err := service.validateDependencies(); err != nil {
		return Category{}, err
	}
	if err := validateID(command.ID); err != nil {
		return Category{}, err
	}
	language, description, err := normalizeCategory(command.Language, command.Description)
	if err != nil {
		return Category{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Category{}, err
	}
	value := Category{ID: command.ID, Language: language, Description: description}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, err
	}
	return value, nil
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, id int64) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Delete(ctx, id)
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}
