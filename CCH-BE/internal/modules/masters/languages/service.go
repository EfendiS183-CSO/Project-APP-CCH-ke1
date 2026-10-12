package languages

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Language, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, code string) (Language, error) {
	if err := service.validateDependencies(); err != nil {
		return Language{}, err
	}
	code, err := normalizeCode(code)
	if err != nil {
		return Language{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Language{}, err
	}
	value, err := service.repository.Get(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return Language{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Language, error) {
	if err := service.validateDependencies(); err != nil {
		return Language{}, err
	}
	value, err := normalizeCreate(command)
	if err != nil {
		return Language{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Language{}, err
	}
	if err := service.repository.Insert(ctx, value); err != nil {
		return Language{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Language, error) {
	if err := service.validateDependencies(); err != nil {
		return Language{}, err
	}
	value, err := normalizeUpdate(command)
	if err != nil {
		return Language{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Language{}, err
	}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Language{}, ErrNotFound
		}
		return Language{}, err
	}
	return value, nil
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, code string) error {
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
	return service.repository.Delete(ctx, code)
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}

func normalizeCreate(command CreateCommand) (Language, error) {
	code, err := normalizeCode(command.Code)
	if err != nil {
		return Language{}, err
	}
	return buildLanguage(code, command.Name, command.IsDefault, command.SiteLogo, command.SiteTitle,
		command.DefaultThousandsSeparator, command.DefaultDecimalPoint, command.DefaultCurrencySymbol,
		command.DefaultMoneyThousandsSep, command.DefaultMoneyDecimalPoint,
		command.TermsAndConditionText, command.AnnouncementText, command.AboutText)
}

func normalizeUpdate(command UpdateCommand) (Language, error) {
	code, err := normalizeCode(command.Code)
	if err != nil {
		return Language{}, err
	}
	return buildLanguage(code, command.Name, command.IsDefault, command.SiteLogo, command.SiteTitle,
		command.DefaultThousandsSeparator, command.DefaultDecimalPoint, command.DefaultCurrencySymbol,
		command.DefaultMoneyThousandsSep, command.DefaultMoneyDecimalPoint,
		command.TermsAndConditionText, command.AnnouncementText, command.AboutText)
}

func buildLanguage(code, name string, isDefault bool, siteLogo, siteTitle string,
	thousands, decimal, currency, moneyThousands, moneyDecimal *string,
	terms, announcement, about string) (Language, error) {
	name, err := normalizeName(name)
	if err != nil {
		return Language{}, err
	}
	siteLogo, err = normalizeSiteText(siteLogo)
	if err != nil {
		return Language{}, err
	}
	siteTitle, err = normalizeSiteText(siteTitle)
	if err != nil {
		return Language{}, err
	}
	terms, err = normalizeBodyText(terms)
	if err != nil {
		return Language{}, err
	}
	announcement, err = normalizeBodyText(announcement)
	if err != nil {
		return Language{}, err
	}
	about, err = normalizeBodyText(about)
	if err != nil {
		return Language{}, err
	}
	thousands, err = normalizeOptionalText(thousands, 5)
	if err != nil {
		return Language{}, err
	}
	decimal, err = normalizeOptionalText(decimal, 5)
	if err != nil {
		return Language{}, err
	}
	currency, err = normalizeOptionalText(currency, 10)
	if err != nil {
		return Language{}, err
	}
	moneyThousands, err = normalizeOptionalText(moneyThousands, 5)
	if err != nil {
		return Language{}, err
	}
	moneyDecimal, err = normalizeOptionalText(moneyDecimal, 5)
	if err != nil {
		return Language{}, err
	}
	return Language{
		Code: code, Name: name, IsDefault: isDefault,
		SiteLogo: siteLogo, SiteTitle: siteTitle,
		DefaultThousandsSeparator: thousands, DefaultDecimalPoint: decimal,
		DefaultCurrencySymbol: currency, DefaultMoneyThousandsSep: moneyThousands,
		DefaultMoneyDecimalPoint: moneyDecimal,
		TermsAndConditionText:    terms, AnnouncementText: announcement, AboutText: about,
	}, nil
}
