package sources

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type Service struct {
	authorization *authorization.AccessService
	repository    Repository
	now           func() time.Time
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{authorization: accessService, repository: repository, now: time.Now}
}

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Source, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, id int64) (Source, error) {
	if err := service.validateDependencies(); err != nil {
		return Source{}, err
	}
	if err := validateID(id); err != nil {
		return Source{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Source{}, err
	}
	source, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	return source, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Source, error) {
	if err := service.validateDependencies(); err != nil {
		return Source{}, err
	}
	name, err := normalizeName(command.Name)
	if err != nil {
		return Source{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Source{}, err
	}
	now := service.now().UTC()
	return service.repository.Insert(ctx, Source{Name: name, Active: command.Active, CreatedAt: &now})
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Source, error) {
	if err := service.validateDependencies(); err != nil {
		return Source{}, err
	}
	if err := validateID(command.ID); err != nil {
		return Source{}, err
	}
	name, err := normalizeName(command.Name)
	if err != nil {
		return Source{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Source{}, err
	}
	updatedAt := service.now().UTC()
	source := Source{ID: command.ID, Name: name, Active: command.Active, UpdatedAt: &updatedAt}
	if err := service.repository.Update(ctx, source); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Source{}, ErrNotFound
		}
		return Source{}, err
	}
	return source, nil
}

func (service *Service) Deactivate(ctx context.Context, identity authentication.VerifiedIdentity, id int64) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Deactivate(ctx, id, service.now().UTC())
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil || service.now == nil {
		return ErrServiceNotReady
	}
	return nil
}
