package customers

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidProfile         = errors.New("invalid customer profile")
	ErrProfileAlreadyExists   = errors.New("customer name already exists")
	ErrProfileInUse           = errors.New("customer profile is referenced by ticket or intake draft")
	ErrProfileServiceNotReady = errors.New("customer profile service is not configured")
)

const (
	defaultProfilePageSize = 25
	maxProfilePageSize     = 100
	maxProfileAddressRunes = 5000
)

type ProfileListQuery struct {
	OfficeCode string
	Limit      int
	Offset     int
}

type ProfilePage struct {
	Data   []Profile `json:"data"`
	Total  int64     `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

func (query ProfileListQuery) Normalize() (ProfileListQuery, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return ProfileListQuery{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return ProfileListQuery{}, ErrInvalidProfile
	}
	return query, nil
}

func (query ProfileListQuery) NormalizeRequest() (ProfileListQuery, error) {
	if query.Limit == 0 {
		query.Limit = defaultProfilePageSize
	}
	if query.Limit < 1 || query.Limit > maxProfilePageSize || query.Offset < 0 {
		return ProfileListQuery{}, ErrInvalidProfile
	}
	return query, nil
}

type CreateProfileCommand struct {
	FullName string
	Email    string
	Phone    string
	Address  string
}

type UpdateProfileCommand struct {
	ID       int64
	FullName string
	Email    string
	Phone    string
	Address  string
}

type ProfileRepository interface {
	ListProfiles(context.Context, ProfileListQuery) (ProfilePage, error)
	GetProfile(context.Context, int64, string) (Profile, error)
	CreateProfile(context.Context, Profile, string, string) (Profile, error)
	UpdateProfile(context.Context, int64, string, Profile, string, string) (Profile, error)
	DeleteProfile(context.Context, int64, string) error
}

func normalizeProfile(fullName string, email string, phone string, address string) (Profile, error) {
	fullName = strings.ToUpper(strings.TrimSpace(fullName))
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	address = strings.ToUpper(strings.TrimSpace(address))
	if fullName == "" || utf8.RuneCountInString(fullName) > 50 ||
		email == "" || utf8.RuneCountInString(email) > 100 ||
		phone == "" || utf8.RuneCountInString(phone) > 100 ||
		address == "" || utf8.RuneCountInString(address) > maxProfileAddressRunes ||
		!utf8.ValidString(fullName) || !utf8.ValidString(email) || !utf8.ValidString(phone) || !utf8.ValidString(address) {
		return Profile{}, ErrInvalidProfile
	}
	return Profile{FullName: fullName, Email: email, Phone: phone, Address: address}, nil
}
