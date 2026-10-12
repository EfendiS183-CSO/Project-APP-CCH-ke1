package followups

import (
	"context"
	"errors"
	"strings"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

var (
	ErrInvalidPlanRequest       = errors.New("invalid follow-up plan request")
	ErrPlanNotAvailable         = errors.New("follow-up plan is not available for this complaint")
	ErrPlanAlreadyExists        = errors.New("follow-up plan already exists")
	ErrPlanCancellationNotFound = errors.New("follow-up plan was not found")
	ErrPlanCancellationConflict = errors.New("follow-up plan was already realized")
	ErrPlanEditNotFound         = errors.New("follow-up plan was not found")
	ErrPlanEditConflict         = errors.New("follow-up plan was already realized")
	ErrInvalidRealization       = errors.New("invalid follow-up realization")
	ErrRealizationConflict      = errors.New("follow-up was already realized with different values")
	ErrRealizationNotFound      = errors.New("follow-up was not found in the current office")
)

type CreatePlanRequest struct {
	ComplaintID string
	StatusCode  string
	PlannedAt   time.Time
	PlanNotes   string
}

func (request CreatePlanRequest) Normalize() (CreatePlanRequest, error) {
	request.ComplaintID = strings.TrimSpace(request.ComplaintID)
	request.StatusCode = strings.TrimSpace(request.StatusCode)
	request.PlanNotes = strings.TrimSpace(request.PlanNotes)
	if request.ComplaintID == "" || len(request.ComplaintID) > 25 || strings.Contains(request.ComplaintID, ",") ||
		len(request.StatusCode) != 3 || request.PlannedAt.IsZero() || len([]rune(request.PlanNotes)) > 5000 {
		return CreatePlanRequest{}, ErrInvalidPlanRequest
	}
	for _, character := range request.StatusCode {
		if !((character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return CreatePlanRequest{}, ErrInvalidPlanRequest
		}
	}
	request.PlannedAt = request.PlannedAt.UTC().Truncate(time.Second)
	return request, nil
}

type CreatePlanCommand struct {
	CreatePlanRequest
	OfficeCode string
	Actor      string
}

type CancelPlanRequest struct {
	ComplaintID string
	StatusCode  string
}

func (request CancelPlanRequest) Normalize() (CancelPlanRequest, error) {
	request.ComplaintID = strings.TrimSpace(request.ComplaintID)
	request.StatusCode = strings.TrimSpace(request.StatusCode)
	if request.ComplaintID == "" || len(request.ComplaintID) > 25 || strings.Contains(request.ComplaintID, ",") || len(request.StatusCode) != 3 {
		return CancelPlanRequest{}, ErrInvalidPlanRequest
	}
	for _, character := range request.StatusCode {
		if !((character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return CancelPlanRequest{}, ErrInvalidPlanRequest
		}
	}
	return request, nil
}

type CancelPlanCommand struct {
	CancelPlanRequest
	OfficeCode string
}

type EditPlanRequest struct {
	ComplaintID string
	StatusCode  string
	PlannedAt   time.Time
	PlanNotes   string
}

func (request EditPlanRequest) Normalize() (EditPlanRequest, error) {
	normalized, err := (CreatePlanRequest{
		ComplaintID: request.ComplaintID,
		StatusCode:  request.StatusCode,
		PlannedAt:   request.PlannedAt,
		PlanNotes:   request.PlanNotes,
	}).Normalize()
	if err != nil {
		return EditPlanRequest{}, err
	}
	return EditPlanRequest{ComplaintID: normalized.ComplaintID, StatusCode: normalized.StatusCode, PlannedAt: normalized.PlannedAt, PlanNotes: normalized.PlanNotes}, nil
}

type EditPlanCommand struct {
	EditPlanRequest
	OfficeCode string
}

type PlanRepository interface {
	CreateCallCenterPlan(context.Context, CreatePlanCommand) (Entry, error)
}

type PlanCancellationRepository interface {
	CancelCallCenterPlan(context.Context, CancelPlanCommand) error
}

type PlanEditRepository interface {
	EditCallCenterPlan(context.Context, EditPlanCommand) (Entry, error)
}

type RealizeRequest struct {
	ComplaintID string
	StatusCode  string
	Outcome     string
}

func (request RealizeRequest) Normalize() (RealizeRequest, error) {
	request.ComplaintID = strings.TrimSpace(request.ComplaintID)
	request.StatusCode = strings.TrimSpace(request.StatusCode)
	request.Outcome = strings.TrimSpace(request.Outcome)
	if request.ComplaintID == "" || len(request.ComplaintID) > 25 || strings.Contains(request.ComplaintID, ",") ||
		len(request.StatusCode) != 3 || request.Outcome == "" || len([]rune(request.Outcome)) > 5000 {
		return RealizeRequest{}, ErrInvalidRealization
	}
	for _, character := range request.StatusCode {
		if !((character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return RealizeRequest{}, ErrInvalidRealization
		}
	}
	return request, nil
}

type RealizeCommand struct {
	RealizeRequest
	OfficeCode string
	Actor      string
}

type RealizeResult struct {
	Entry    Entry
	Replayed bool
}

type RealizationRepository interface {
	RealizeCallCenterPlan(context.Context, RealizeCommand) (RealizeResult, error)
}

type CallCenterPlanService struct {
	accessService *authorization.AccessService
	repository    PlanRepository
}

func NewCallCenterPlanService(accessService *authorization.AccessService, repository PlanRepository) *CallCenterPlanService {
	return &CallCenterPlanService{accessService: accessService, repository: repository}
}

func (service *CallCenterPlanService) Create(ctx context.Context, identity authentication.VerifiedIdentity, request CreatePlanRequest) (Entry, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return Entry{}, ErrServiceNotConfigured
	}
	request, err := request.Normalize()
	if err != nil {
		return Entry{}, err
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return Entry{}, err
	}
	followUpAccess, err := service.accessService.Authorize(ctx, identity, followUpTableName, authorization.ActionAdd)
	if err != nil {
		return Entry{}, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(followUpAccess.UserLevelIDs) {
		return Entry{}, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != followUpAccess.OfficeCode ||
		complaintAccess.ReportTo != followUpAccess.ReportTo ||
		!sameUserLevels(complaintAccess.UserLevelIDs, followUpAccess.UserLevelIDs) {
		return Entry{}, authorization.ErrIdentityMismatch
	}
	if complaintAccess.Username == "" || len(complaintAccess.Username) > 20 {
		return Entry{}, authorization.ErrIdentityMismatch
	}
	return service.repository.CreateCallCenterPlan(ctx, CreatePlanCommand{
		CreatePlanRequest: request,
		OfficeCode:        complaintAccess.OfficeCode,
		Actor:             complaintAccess.Username,
	})
}

type CallCenterPlanCancellationService struct {
	accessService *authorization.AccessService
	repository    PlanCancellationRepository
}

func NewCallCenterPlanCancellationService(accessService *authorization.AccessService, repository PlanCancellationRepository) *CallCenterPlanCancellationService {
	return &CallCenterPlanCancellationService{accessService: accessService, repository: repository}
}

func (service *CallCenterPlanCancellationService) Cancel(ctx context.Context, identity authentication.VerifiedIdentity, request CancelPlanRequest) error {
	if service == nil || service.accessService == nil || service.repository == nil {
		return ErrServiceNotConfigured
	}
	request, err := request.Normalize()
	if err != nil {
		return err
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return err
	}
	followUpAccess, err := service.accessService.Authorize(ctx, identity, followUpTableName, authorization.ActionDelete)
	if err != nil {
		return err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(followUpAccess.UserLevelIDs) {
		return ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != followUpAccess.OfficeCode ||
		complaintAccess.ReportTo != followUpAccess.ReportTo || !sameUserLevels(complaintAccess.UserLevelIDs, followUpAccess.UserLevelIDs) {
		return authorization.ErrIdentityMismatch
	}
	return service.repository.CancelCallCenterPlan(ctx, CancelPlanCommand{CancelPlanRequest: request, OfficeCode: complaintAccess.OfficeCode})
}

type CallCenterPlanEditService struct {
	accessService *authorization.AccessService
	repository    PlanEditRepository
}

func NewCallCenterPlanEditService(accessService *authorization.AccessService, repository PlanEditRepository) *CallCenterPlanEditService {
	return &CallCenterPlanEditService{accessService: accessService, repository: repository}
}

func (service *CallCenterPlanEditService) Edit(ctx context.Context, identity authentication.VerifiedIdentity, request EditPlanRequest) (Entry, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return Entry{}, ErrServiceNotConfigured
	}
	request, err := request.Normalize()
	if err != nil {
		return Entry{}, err
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return Entry{}, err
	}
	followUpAccess, err := service.accessService.Authorize(ctx, identity, followUpTableName, authorization.ActionEdit)
	if err != nil {
		return Entry{}, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(followUpAccess.UserLevelIDs) {
		return Entry{}, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != followUpAccess.OfficeCode ||
		complaintAccess.ReportTo != followUpAccess.ReportTo || !sameUserLevels(complaintAccess.UserLevelIDs, followUpAccess.UserLevelIDs) {
		return Entry{}, authorization.ErrIdentityMismatch
	}
	return service.repository.EditCallCenterPlan(ctx, EditPlanCommand{EditPlanRequest: request, OfficeCode: complaintAccess.OfficeCode})
}

type CallCenterRealizationService struct {
	accessService *authorization.AccessService
	repository    RealizationRepository
}

func NewCallCenterRealizationService(accessService *authorization.AccessService, repository RealizationRepository) *CallCenterRealizationService {
	return &CallCenterRealizationService{accessService: accessService, repository: repository}
}

func (service *CallCenterRealizationService) Realize(ctx context.Context, identity authentication.VerifiedIdentity, request RealizeRequest) (RealizeResult, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return RealizeResult{}, ErrServiceNotConfigured
	}
	request, err := request.Normalize()
	if err != nil {
		return RealizeResult{}, err
	}
	access, err := service.accessService.Authorize(ctx, identity, followUpTableName, authorization.ActionEdit)
	if err != nil {
		return RealizeResult{}, err
	}
	if !isSingleCallCenterRole(access.UserLevelIDs) {
		return RealizeResult{}, ErrCallCenterRoleRequired
	}
	if access.OfficeCode == "" || len(access.OfficeCode) > 7 || access.Username == "" || len(access.Username) > 20 {
		return RealizeResult{}, authorization.ErrIdentityMismatch
	}
	return service.repository.RealizeCallCenterPlan(ctx, RealizeCommand{
		RealizeRequest: request,
		OfficeCode:     access.OfficeCode,
		Actor:          access.Username,
	})
}

func sameUserLevels(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
