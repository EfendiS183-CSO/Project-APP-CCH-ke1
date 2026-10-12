package holidaycalendar

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Holiday, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, date string) (Holiday, error) {
	if err := service.validateDependencies(); err != nil {
		return Holiday{}, err
	}
	date, err := NormalizeDate(date)
	if err != nil {
		return Holiday{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Holiday{}, err
	}
	value, err := service.repository.Get(ctx, date)
	if errors.Is(err, sql.ErrNoRows) {
		return Holiday{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Holiday, error) {
	if err := service.validateDependencies(); err != nil {
		return Holiday{}, err
	}
	date, err := NormalizeDate(command.Date)
	if err != nil {
		return Holiday{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return Holiday{}, err
	}
	actor, err := normalizeActor(identity.Username)
	if err != nil {
		return Holiday{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Holiday{}, err
	}
	createdAt := service.now().UTC()
	value := Holiday{Date: date, Description: description, CreatedAt: &createdAt, CreatedBy: actor}
	if err := service.repository.Insert(ctx, value); err != nil {
		return Holiday{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Holiday, error) {
	if err := service.validateDependencies(); err != nil {
		return Holiday{}, err
	}
	date, err := NormalizeDate(command.Date)
	if err != nil {
		return Holiday{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return Holiday{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Holiday{}, err
	}
	value := Holiday{Date: date, Description: description}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Holiday{}, ErrNotFound
		}
		return Holiday{}, err
	}
	return value, nil
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, date string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	date, err := NormalizeDate(date)
	if err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Delete(ctx, date)
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil || service.now == nil {
		return ErrServiceNotReady
	}
	return nil
}
