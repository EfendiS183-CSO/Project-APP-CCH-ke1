package documenttypes

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]DocumentType, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, code string) (DocumentType, error) {
	if err := service.validateDependencies(); err != nil {
		return DocumentType{}, err
	}
	code, err := normalizeCode(code)
	if err != nil {
		return DocumentType{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return DocumentType{}, err
	}
	value, err := service.repository.Get(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentType{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (DocumentType, error) {
	if err := service.validateDependencies(); err != nil {
		return DocumentType{}, err
	}
	code, err := normalizeCode(command.Code)
	if err != nil {
		return DocumentType{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return DocumentType{}, err
	}
	actor, err := normalizeActor(identity.Username)
	if err != nil {
		return DocumentType{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return DocumentType{}, err
	}
	createdAt := service.now().UTC()
	value := DocumentType{Code: code, Description: description, Active: command.Active, CreatedAt: &createdAt, CreatedBy: actor, UpdatedAt: &createdAt, UpdatedBy: actor}
	if err := service.repository.Insert(ctx, value); err != nil {
		return DocumentType{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (DocumentType, error) {
	if err := service.validateDependencies(); err != nil {
		return DocumentType{}, err
	}
	code, err := normalizeCode(command.Code)
	if err != nil {
		return DocumentType{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return DocumentType{}, err
	}
	actor, err := normalizeActor(identity.Username)
	if err != nil {
		return DocumentType{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return DocumentType{}, err
	}
	updatedAt := service.now().UTC()
	value := DocumentType{Code: code, Description: description, Active: command.Active, UpdatedAt: &updatedAt, UpdatedBy: actor}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DocumentType{}, ErrNotFound
		}
		return DocumentType{}, err
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
	actor, err := normalizeActor(identity.Username)
	if err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Deactivate(ctx, code, actor, service.now().UTC())
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil || service.now == nil {
		return ErrServiceNotReady
	}
	return nil
}
