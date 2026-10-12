package complaints

import (
	"context"
	"errors"
	"strings"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

const callCenterLookupRoleID = 3

var (
	ErrLookupServiceNotConfigured   = errors.New("complaint lookup service is not configured")
	ErrLookupCallCenterRoleRequired = errors.New("Call Center role is required for complaint lookups")
	ErrLookupOfficeRequired         = errors.New("current CCH office is required for complaint lookups")
)

type ComplaintTypeOption struct {
	Code        int64  `json:"code"`
	Description string `json:"description"`
}

type ComplaintSourceOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type OfficeOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ShippingServiceOption struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type CallCenterComplaintLookups struct {
	ComplaintTypes     []ComplaintTypeOption   `json:"complaint_types"`
	Sources            []ComplaintSourceOption `json:"sources"`
	OriginOfficeCode   string                  `json:"origin_office_code"`
	DestinationOffices []OfficeOption          `json:"destination_offices"`
	CopyOffices        []OfficeOption          `json:"copy_offices"`
	ShippingServices   []ShippingServiceOption `json:"shipping_services"`
	ShippingOffices    []OfficeOption          `json:"shipping_offices"`
}

type LookupRepository interface {
	ListActiveComplaintTypes(context.Context) ([]ComplaintTypeOption, error)
	ListActiveComplaintSources(context.Context) ([]ComplaintSourceOption, error)
	ListDestinationOffices(context.Context, string) ([]OfficeOption, error)
	ListCopyOffices(context.Context, string) ([]OfficeOption, error)
	ListShippingServices(context.Context) ([]ShippingServiceOption, error)
	ListShippingOffices(context.Context) ([]OfficeOption, error)
}

type CallCenterLookupService struct {
	accessService *authorization.AccessService
	repository    LookupRepository
}

func NewCallCenterLookupService(accessService *authorization.AccessService, repository LookupRepository) *CallCenterLookupService {
	return &CallCenterLookupService{accessService: accessService, repository: repository}
}

func (service *CallCenterLookupService) Get(ctx context.Context, identity authentication.VerifiedIdentity) (CallCenterComplaintLookups, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return CallCenterComplaintLookups{}, ErrLookupServiceNotConfigured
	}
	access, err := service.accessService.Authorize(ctx, identity, cchComplaintTable, authorization.ActionLookup)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	if len(access.UserLevelIDs) != 1 || access.UserLevelIDs[0] != callCenterLookupRoleID {
		return CallCenterComplaintLookups{}, ErrLookupCallCenterRoleRequired
	}
	officeCode := strings.TrimSpace(access.OfficeCode)
	if officeCode == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return CallCenterComplaintLookups{}, ErrLookupOfficeRequired
	}
	complaintTypes, err := service.repository.ListActiveComplaintTypes(ctx)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	sources, err := service.repository.ListActiveComplaintSources(ctx)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	destinationOffices, err := service.repository.ListDestinationOffices(ctx, officeCode)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	copyOffices, err := service.repository.ListCopyOffices(ctx, officeCode)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	shippingServices, err := service.repository.ListShippingServices(ctx)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	shippingOffices, err := service.repository.ListShippingOffices(ctx)
	if err != nil {
		return CallCenterComplaintLookups{}, err
	}
	return CallCenterComplaintLookups{
		ComplaintTypes:     complaintTypes,
		Sources:            sources,
		OriginOfficeCode:   officeCode,
		DestinationOffices: destinationOffices,
		CopyOffices:        copyOffices,
		ShippingServices:   shippingServices,
		ShippingOffices:    shippingOffices,
	}, nil
}
