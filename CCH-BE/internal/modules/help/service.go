package help

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
	query, err := normalizeListQuery(query)
	if err != nil {
		return Page{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return Page{}, err
	}
	return service.repository.List(ctx, query)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, id int64) (Article, error) {
	if err := service.validateDependencies(); err != nil {
		return Article{}, err
	}
	if id < 1 {
		return Article{}, ErrInvalidInput
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Article{}, err
	}
	article, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	return article, err
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

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Article, error) {
	if err := service.validateDependencies(); err != nil {
		return Article{}, err
	}
	article, actor, err := normalizeArticle(command, 0, identity.Username)
	if err != nil {
		return Article{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Article{}, err
	}
	return service.repository.Insert(ctx, article, actor)
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Article, error) {
	if err := service.validateDependencies(); err != nil {
		return Article{}, err
	}
	article, actor, err := normalizeArticle(command.CreateCommand, command.ID, identity.Username)
	if err != nil || command.ID < 1 {
		return Article{}, ErrInvalidInput
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Article{}, err
	}
	value, err := service.repository.Update(ctx, article, actor)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, id int64) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	if id < 1 {
		return ErrInvalidInput
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	err := service.repository.Delete(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}
