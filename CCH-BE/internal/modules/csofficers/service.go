package csofficers

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

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, username string) (Officer, error) {
	if err := service.validateDependencies(); err != nil {
		return Officer{}, err
	}
	username, err := NormalizeUsername(username)
	if err != nil {
		return Officer{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Officer{}, err
	}
	value, err := service.repository.Get(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return Officer{}, ErrNotFound
	}
	return value, err
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}
