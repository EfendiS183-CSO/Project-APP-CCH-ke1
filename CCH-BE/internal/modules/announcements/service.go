package announcements

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type Service struct {
	accessService *authorization.AccessService
	repository    Repository
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{accessService: accessService, repository: repository}
}

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity, query ListQuery) (Page, error) {
	if err := service.validateDependencies(); err != nil {
		return Page{}, err
	}
	query, err := query.Normalize()
	if err != nil {
		return Page{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return Page{}, err
	}
	return service.repository.List(ctx, query)
}

func (service *Service) Options(ctx context.Context, identity authentication.VerifiedIdentity) (Options, error) {
	if err := service.validateDependencies(); err != nil {
		return Options{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionLookup); err != nil {
		return Options{}, err
	}
	return service.repository.Options(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, id int64) (Announcement, error) {
	if err := service.validateDependencies(); err != nil {
		return Announcement{}, err
	}
	if id <= 0 {
		return Announcement{}, ErrInvalidInput
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Announcement{}, err
	}
	value, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Announcement{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Announcement, error) {
	if err := service.validateDependencies(); err != nil {
		return Announcement{}, err
	}
	command, err := command.Normalize()
	if err != nil {
		return Announcement{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Announcement{}, err
	}
	exists, err := service.repository.LanguageExists(ctx, command.Language)
	if err != nil {
		return Announcement{}, err
	}
	if !exists {
		return Announcement{}, ErrInvalidLanguage
	}
	if command.TranslatedID != nil && *command.TranslatedID > 0 {
		count, err := service.repository.CountByLanguage(ctx, command.Language, *command.TranslatedID)
		if err != nil {
			return Announcement{}, err
		}
		if count > 0 {
			return Announcement{}, ErrDuplicateTranslation
		}
	}
	if command.DateStart != nil && command.DateEnd != nil {
		overlaps, err := service.repository.FindOverlap(ctx, command.Language, *command.DateStart, *command.DateEnd, 0)
		if err != nil {
			return Announcement{}, err
		}
		if overlaps {
			return Announcement{}, ErrDateOverlaps
		}
	}
	value := Announcement{
		IsActive:     command.IsActive,
		Topic:        command.Topic,
		Message:      command.Message,
		Language:     command.Language,
		AutoPublish:  command.AutoPublish,
		DateStart:    command.DateStart,
		DateEnd:      command.DateEnd,
		TranslatedID: command.TranslatedID,
	}
	return service.repository.Create(ctx, value, actorName(identity))
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Announcement, error) {
	if err := service.validateDependencies(); err != nil {
		return Announcement{}, err
	}
	command, err := command.Normalize()
	if err != nil {
		return Announcement{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Announcement{}, err
	}
	exists, err := service.repository.LanguageExists(ctx, command.Language)
	if err != nil {
		return Announcement{}, err
	}
	if !exists {
		return Announcement{}, ErrInvalidLanguage
	}
	if command.DateStart != nil && command.DateEnd != nil {
		overlaps, err := service.repository.FindOverlap(ctx, command.Language, *command.DateStart, *command.DateEnd, command.ID)
		if err != nil {
			return Announcement{}, err
		}
		if overlaps {
			return Announcement{}, ErrDateOverlaps
		}
	}
	value := Announcement{
		ID:           command.ID,
		IsActive:     command.IsActive,
		Topic:        command.Topic,
		Message:      command.Message,
		Language:     command.Language,
		AutoPublish:  command.AutoPublish,
		DateStart:    command.DateStart,
		DateEnd:      command.DateEnd,
		TranslatedID: command.TranslatedID,
	}
	return service.repository.Update(ctx, value, actorName(identity))
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, id int64) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	if id <= 0 {
		return ErrInvalidInput
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Delete(ctx, id)
}

func (service *Service) validateDependencies() error {
	if service == nil || service.accessService == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}

func actorName(identity authentication.VerifiedIdentity) string {
	name := strings.TrimSpace(identity.Username)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 200 {
		return ""
	}
	return name
}
