package followups

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

const (
	complaintTableName       = "cchentri"
	followUpViewName         = "cchfual"
	followUpTableName        = "cchfu"
	followUpDueTodayViewName = "cchfuhi"
	callCenterRoleID         = 3
)

type Service struct {
	accessService *authorization.AccessService
	repository    Repository
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{accessService: accessService, repository: repository}
}

func (service *Service) ListForCallCenterTicket(ctx context.Context, identity authentication.VerifiedIdentity, query Query) ([]Entry, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return nil, ErrServiceNotConfigured
	}
	query.OfficeCode = ""
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return nil, err
	}
	followUpAccess, err := service.accessService.Authorize(ctx, identity, followUpViewName, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(followUpAccess.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != followUpAccess.OfficeCode ||
		complaintAccess.ReportTo != followUpAccess.ReportTo ||
		!slices.Equal(complaintAccess.UserLevelIDs, followUpAccess.UserLevelIDs) {
		return nil, authorization.ErrIdentityMismatch
	}

	query.OfficeCode = complaintAccess.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return nil, err
	}
	entries, err := service.repository.ListForOpenCallCenterTicket(ctx, query)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFoundOrNotVisible
	}
	return entries, err
}

func (service *Service) ListForCallCenterOffice(ctx context.Context, identity authentication.VerifiedIdentity, query OfficeQuery) ([]Entry, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return nil, ErrServiceNotConfigured
	}
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}
	access, err := service.accessService.Authorize(ctx, identity, followUpTableName, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isSingleCallCenterRole(access.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	query.OfficeCode = access.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return nil, err
	}
	return service.repository.ListForCallCenterOffice(ctx, query)
}

func (service *Service) ListDueTodayForCallCenterOffice(ctx context.Context, identity authentication.VerifiedIdentity, query OfficeQuery) ([]Entry, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return nil, ErrServiceNotConfigured
	}
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}
	access, err := service.accessService.Authorize(ctx, identity, followUpDueTodayViewName, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isSingleCallCenterRole(access.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	query.OfficeCode = access.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return nil, ErrCallCenterOfficeRequired
	}
	return service.repository.ListDueTodayForCallCenterOffice(ctx, query)
}

func isSingleCallCenterRole(userLevelIDs []int64) bool {
	return len(userLevelIDs) == 1 && userLevelIDs[0] == callCenterRoleID
}
