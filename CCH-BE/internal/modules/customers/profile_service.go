package customers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type ProfileService struct {
	accessService *authorization.AccessService
	repository    ProfileRepository
	// now           func() time.Time
}

func NewProfileService(accessService *authorization.AccessService, repository ProfileRepository) *ProfileService {
	return &ProfileService{accessService: accessService, repository: repository}
}

func (service *ProfileService) List(ctx context.Context, identity authentication.VerifiedIdentity, query ProfileListQuery) (ProfilePage, error) {
	if err := service.validateDependencies(); err != nil {
		return ProfilePage{}, err
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionList)
	if err != nil {
		return ProfilePage{}, err
	}
	query.OfficeCode = access.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return ProfilePage{}, ErrOfficeRequired
	}
	return service.repository.ListProfiles(ctx, query)
}

func (service *ProfileService) Get(ctx context.Context, identity authentication.VerifiedIdentity, profileID int64) (Profile, error) {
	if err := service.validateDependencies(); err != nil {
		return Profile{}, err
	}
	if profileID <= 0 {
		return Profile{}, ErrInvalidProfileID
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionView)
	if err != nil {
		return Profile{}, err
	}
	officeCode, err := normalizeProfileOffice(access.OfficeCode)
	if err != nil {
		return Profile{}, err
	}
	profile, err := service.repository.GetProfile(ctx, profileID, officeCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	return profile, err
}

func (service *ProfileService) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateProfileCommand) (Profile, error) {
	if err := service.validateDependencies(); err != nil {
		return Profile{}, err
	}
	profile, err := normalizeProfile(command.FullName, command.Email, command.Phone, command.Address)
	if err != nil {
		return Profile{}, err
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionAdd)
	if err != nil {
		return Profile{}, err
	}
	officeCode, err := normalizeProfileOffice(access.OfficeCode)
	if err != nil {
		return Profile{}, err
	}
	actor, err := normalizeProfileActor(identity.Username)
	if err != nil {
		return Profile{}, err
	}
	return service.repository.CreateProfile(ctx, profile, officeCode, actor)
}

func (service *ProfileService) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateProfileCommand) (Profile, error) {
	if err := service.validateDependencies(); err != nil {
		return Profile{}, err
	}
	if command.ID <= 0 {
		return Profile{}, ErrInvalidProfile
	}
	profile, err := normalizeProfile(command.FullName, command.Email, command.Phone, command.Address)
	if err != nil {
		return Profile{}, err
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionEdit)
	if err != nil {
		return Profile{}, err
	}
	officeCode, err := normalizeProfileOffice(access.OfficeCode)
	if err != nil {
		return Profile{}, err
	}
	actor, err := normalizeProfileActor(identity.Username)
	if err != nil {
		return Profile{}, err
	}
	profile, err = service.repository.UpdateProfile(ctx, command.ID, officeCode, profile, officeCode, actor)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	return profile, err
}

func (service *ProfileService) Delete(ctx context.Context, identity authentication.VerifiedIdentity, profileID int64) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	if profileID <= 0 {
		return ErrInvalidProfileID
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionDelete)
	if err != nil {
		return err
	}
	officeCode, err := normalizeProfileOffice(access.OfficeCode)
	if err != nil {
		return err
	}
	return service.repository.DeleteProfile(ctx, profileID, officeCode)
}

func (service *ProfileService) validateDependencies() error {
	if service == nil || service.accessService == nil || service.repository == nil {
		return ErrProfileServiceNotReady
	}
	return nil
}

func normalizeProfileOffice(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 7 || strings.Contains(value, ",") {
		return "", ErrOfficeRequired
	}
	return value, nil
}

func normalizeProfileActor(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 20 {
		return "", ErrInvalidProfile
	}
	return value, nil
}
