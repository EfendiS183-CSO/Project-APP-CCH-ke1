package complaints

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

const cchComplaintTable = "cchentri"
const callCenterUserLevelID int64 = 3

var (
	ErrComplaintServiceNotConfigured = errors.New("complaint service is not configured")
	ErrCallCenterRoleRequired        = errors.New("Call Center role is required")
	ErrDashboardRoleRequired         = errors.New("a supported single office role is required for dashboard counts")
)

type Service struct {
	authorization *authorization.AccessService
	repository    ComplaintRepository
}

func NewService(accessService *authorization.AccessService, repository ComplaintRepository) *Service {
	return &Service{authorization: accessService, repository: repository}
}

func (service *Service) ListOpenForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, page ComplaintListQuery) ([]ComplaintSummary, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}

	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if len(access.UserLevelIDs) != 1 || access.UserLevelIDs[0] != callCenterUserLevelID {
		return nil, ErrCallCenterRoleRequired
	}

	page.OfficeCode = access.OfficeCode
	page.OfficeMatch = MatchOriginOrAllTargets
	page.OpenOnly = true
	return service.repository.ListInvolvingOffice(ctx, page)
}

func (service *Service) ListDashboardReceivedOpen(ctx context.Context, identity authentication.VerifiedIdentity, page ComplaintListQuery) ([]ComplaintSummary, error) {
	return service.listDashboardOpen(ctx, identity, page, MatchAllTargetsOnly)
}

func (service *Service) ListDashboardSentOpen(ctx context.Context, identity authentication.VerifiedIdentity, page ComplaintListQuery) ([]ComplaintSummary, error) {
	return service.listDashboardOpen(ctx, identity, page, MatchOriginOnly)
}

func (service *Service) ListReceivedOpenReport(ctx context.Context, identity authentication.VerifiedIdentity, query ReceivedOpenReportQuery) ([]ReceivedOpenReportRow, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return nil, ErrDashboardRoleRequired
	}
	distributionAccess, err := service.authorization.Authorize(ctx, identity, "cchentridet", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	historyAccess, err := service.authorization.Authorize(ctx, identity, "cchentrimas", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, distributionAccess.UserLevelIDs) || !sameRoleIDs(parentAccess.UserLevelIDs, historyAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != distributionAccess.OfficeCode || parentAccess.OfficeCode != historyAccess.OfficeCode ||
		parentAccess.ReportTo != distributionAccess.ReportTo || parentAccess.ReportTo != historyAccess.ReportTo {
		return nil, authorization.ErrIdentityMismatch
	}
	return service.repository.ListReceivedOpenReport(ctx, query, parentAccess.OfficeCode)
}

func (service *Service) ListSentOpenReport(ctx context.Context, identity authentication.VerifiedIdentity, query SentOpenReportQuery) ([]SentOpenReportRow, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return nil, ErrDashboardRoleRequired
	}
	distributionAccess, err := service.authorization.Authorize(ctx, identity, "cchentridet", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	historyAccess, err := service.authorization.Authorize(ctx, identity, "cchentrimas", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, distributionAccess.UserLevelIDs) || !sameRoleIDs(parentAccess.UserLevelIDs, historyAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != distributionAccess.OfficeCode || parentAccess.OfficeCode != historyAccess.OfficeCode ||
		parentAccess.ReportTo != distributionAccess.ReportTo || parentAccess.ReportTo != historyAccess.ReportTo {
		return nil, authorization.ErrIdentityMismatch
	}
	return service.repository.ListSentOpenReport(ctx, query, parentAccess.OfficeCode)
}

func (service *Service) ListSentSummary(ctx context.Context, identity authentication.VerifiedIdentity, query SentSummaryQuery) ([]SentSummaryRow, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	submit, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return nil, ErrDashboardRoleRequired
	}
	officeAccess, err := service.authorization.Authorize(ctx, identity, "cch_kantor", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, officeAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != officeAccess.OfficeCode ||
		parentAccess.ReportTo != officeAccess.ReportTo {
		return nil, authorization.ErrIdentityMismatch
	}
	return service.repository.ListSentSummary(ctx, submit, parentAccess.OfficeCode)
}

func (service *Service) ListReceivedSummary(ctx context.Context, identity authentication.VerifiedIdentity, query ReceivedSummaryQuery) ([]ReceivedSummaryRow, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	submit, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return nil, ErrDashboardRoleRequired
	}
	distributionAccess, err := service.authorization.Authorize(ctx, identity, "cchentridet", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	officeAccess, err := service.authorization.Authorize(ctx, identity, "cch_kantor", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, distributionAccess.UserLevelIDs) || !sameRoleIDs(parentAccess.UserLevelIDs, officeAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != distributionAccess.OfficeCode || parentAccess.OfficeCode != officeAccess.OfficeCode ||
		parentAccess.ReportTo != distributionAccess.ReportTo || parentAccess.ReportTo != officeAccess.ReportTo {
		return nil, authorization.ErrIdentityMismatch
	}
	return service.repository.ListReceivedSummary(ctx, submit, parentAccess.OfficeCode)
}

func (service *Service) ListBreakdown(ctx context.Context, identity authentication.VerifiedIdentity, query BreakdownQuery) ([]BreakdownRow, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	submit, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return nil, ErrDashboardRoleRequired
	}
	officeAccess, err := service.authorization.Authorize(ctx, identity, "cch_kantor", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, officeAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != officeAccess.OfficeCode ||
		parentAccess.ReportTo != officeAccess.ReportTo {
		return nil, authorization.ErrIdentityMismatch
	}
	if submit.Direction == BreakdownDirectionReceived {
		distributionAccess, err := service.authorization.Authorize(ctx, identity, "cchentridet", authorization.ActionList)
		if err != nil {
			return nil, err
		}
		if !sameRoleIDs(parentAccess.UserLevelIDs, distributionAccess.UserLevelIDs) ||
			parentAccess.OfficeCode != distributionAccess.OfficeCode || parentAccess.ReportTo != distributionAccess.ReportTo {
			return nil, authorization.ErrIdentityMismatch
		}
	}
	return service.repository.ListBreakdown(ctx, submit, parentAccess.OfficeCode)
}

func (service *Service) CountReceivedOpenDurationBuckets(ctx context.Context, identity authentication.VerifiedIdentity, query ReceivedOpenDurationQuery) (ReceivedOpenDurationBuckets, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ReceivedOpenDurationBuckets{}, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return ReceivedOpenDurationBuckets{}, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return ReceivedOpenDurationBuckets{}, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return ReceivedOpenDurationBuckets{}, ErrDashboardRoleRequired
	}
	distributionAccess, err := service.authorization.Authorize(ctx, identity, "cchentridet", authorization.ActionList)
	if err != nil {
		return ReceivedOpenDurationBuckets{}, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, distributionAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != distributionAccess.OfficeCode ||
		parentAccess.ReportTo != distributionAccess.ReportTo {
		return ReceivedOpenDurationBuckets{}, authorization.ErrIdentityMismatch
	}
	return service.repository.CountReceivedOpenDurationBuckets(ctx, query, parentAccess.OfficeCode)
}

func (service *Service) CountSentOpenDurationBuckets(ctx context.Context, identity authentication.VerifiedIdentity, query SentOpenDurationQuery) (SentOpenDurationBuckets, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return SentOpenDurationBuckets{}, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return SentOpenDurationBuckets{}, err
	}
	parentAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return SentOpenDurationBuckets{}, err
	}
	if !isOfficeDashboardRole(parentAccess.UserLevelIDs) {
		return SentOpenDurationBuckets{}, ErrDashboardRoleRequired
	}
	distributionAccess, err := service.authorization.Authorize(ctx, identity, "cchentridet", authorization.ActionList)
	if err != nil {
		return SentOpenDurationBuckets{}, err
	}
	historyAccess, err := service.authorization.Authorize(ctx, identity, "cchentrimas", authorization.ActionList)
	if err != nil {
		return SentOpenDurationBuckets{}, err
	}
	if !sameRoleIDs(parentAccess.UserLevelIDs, distributionAccess.UserLevelIDs) || !sameRoleIDs(parentAccess.UserLevelIDs, historyAccess.UserLevelIDs) ||
		parentAccess.OfficeCode == "" || parentAccess.OfficeCode != distributionAccess.OfficeCode || parentAccess.OfficeCode != historyAccess.OfficeCode ||
		parentAccess.ReportTo != distributionAccess.ReportTo || parentAccess.ReportTo != historyAccess.ReportTo {
		return SentOpenDurationBuckets{}, authorization.ErrIdentityMismatch
	}
	return service.repository.CountSentOpenDurationBuckets(ctx, query, parentAccess.OfficeCode)
}

func (service *Service) listDashboardOpen(ctx context.Context, identity authentication.VerifiedIdentity, page ComplaintListQuery, officeMatch ComplaintOfficeMatch) ([]ComplaintSummary, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	page, err := page.NormalizePagination()
	if err != nil {
		return nil, err
	}
	access, err := service.authorizeOfficeDashboard(ctx, identity)
	if err != nil {
		return nil, err
	}
	page.OfficeCode = access.OfficeCode
	page.OfficeMatch = officeMatch
	page.OpenOnly = true
	page.CompletedOnly = false
	page, err = page.Normalize()
	if err != nil {
		return nil, err
	}
	return service.repository.ListInvolvingOffice(ctx, page)
}

func (service *Service) ExportOpenForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, page ComplaintListQuery) ([]ComplaintSummary, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	page, err := page.NormalizePagination()
	if err != nil {
		return nil, err
	}
	listAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	exportAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionExport)
	if err != nil {
		return nil, err
	}
	if !isCallCenter(listAccess.UserLevelIDs) || !isCallCenter(exportAccess.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	if listAccess.OfficeCode == "" || listAccess.OfficeCode != exportAccess.OfficeCode || !sameRoleIDs(listAccess.UserLevelIDs, exportAccess.UserLevelIDs) {
		return nil, authorization.ErrIdentityMismatch
	}
	page.OfficeCode = listAccess.OfficeCode
	page.OfficeMatch = MatchOriginOrAllTargets
	page.OpenOnly = true
	page.CompletedOnly = false
	page, err = page.Normalize()
	if err != nil {
		return nil, err
	}
	return service.repository.ListInvolvingOffice(ctx, page)
}

func (service *Service) ListCompletedForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, page ComplaintListQuery) ([]ComplaintSummary, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if len(access.UserLevelIDs) != 1 || access.UserLevelIDs[0] != callCenterUserLevelID {
		return nil, ErrCallCenterRoleRequired
	}
	page.OfficeCode = access.OfficeCode
	page.OfficeMatch = MatchAllRoutingFields
	page.OpenOnly = false
	page.CompletedOnly = true
	return service.repository.ListInvolvingOffice(ctx, page)
}

func (service *Service) CountCompletedForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, query CompletedCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return 0, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return 0, ErrCallCenterRoleRequired
	}
	return service.repository.CountCompletedForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountOpenForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, query OpenCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return 0, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return 0, ErrCallCenterRoleRequired
	}
	return service.repository.CountOpenForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountReceivedOpenForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, query OpenCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return 0, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return 0, ErrCallCenterRoleRequired
	}
	return service.repository.CountReceivedOpenForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountSentOpenForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, query OpenCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return 0, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return 0, ErrCallCenterRoleRequired
	}
	return service.repository.CountSentOpenForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountDashboardOpen(ctx context.Context, identity authentication.VerifiedIdentity, query OpenCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorizeOfficeDashboard(ctx, identity)
	if err != nil {
		return 0, err
	}
	return service.repository.CountOpenForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountDashboardReceivedOpen(ctx context.Context, identity authentication.VerifiedIdentity, query OpenCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorizeOfficeDashboard(ctx, identity)
	if err != nil {
		return 0, err
	}
	return service.repository.CountReceivedOpenForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountDashboardSentOpen(ctx context.Context, identity authentication.VerifiedIdentity, query OpenCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorizeOfficeDashboard(ctx, identity)
	if err != nil {
		return 0, err
	}
	return service.repository.CountSentOpenForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) CountDashboardCompleted(ctx context.Context, identity authentication.VerifiedIdentity, query CompletedCountQuery) (int64, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return 0, ErrComplaintServiceNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	access, err := service.authorizeOfficeDashboard(ctx, identity)
	if err != nil {
		return 0, err
	}
	return service.repository.CountCompletedForCallCenter(ctx, query, access.OfficeCode)
}

func (service *Service) authorizeOfficeDashboard(ctx context.Context, identity authentication.VerifiedIdentity) (authorization.AccountAccess, error) {
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return authorization.AccountAccess{}, err
	}
	if !isOfficeDashboardRole(access.UserLevelIDs) {
		return authorization.AccountAccess{}, ErrDashboardRoleRequired
	}
	return access, nil
}

func (service *Service) GetOpenForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, complaintID string) (ComplaintDetail, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ComplaintDetail{}, ErrComplaintServiceNotConfigured
	}
	query, err := (ComplaintDetailQuery{ComplaintID: complaintID}).NormalizeRequest()
	if err != nil {
		return ComplaintDetail{}, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionView)
	if err != nil {
		return ComplaintDetail{}, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return ComplaintDetail{}, ErrCallCenterRoleRequired
	}
	query.OfficeCode = access.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return ComplaintDetail{}, err
	}
	detail, err := service.repository.GetOpenTicketForOffice(ctx, query)
	if errors.Is(err, sql.ErrNoRows) {
		return ComplaintDetail{}, ErrComplaintNotFoundOrNotVisible
	}
	return detail, err
}

func (service *Service) GetCompletedForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, complaintID string) (ComplaintDetail, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ComplaintDetail{}, ErrComplaintServiceNotConfigured
	}
	query, err := (ComplaintDetailQuery{ComplaintID: complaintID}).NormalizeRequest()
	if err != nil {
		return ComplaintDetail{}, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionView)
	if err != nil {
		return ComplaintDetail{}, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return ComplaintDetail{}, ErrCallCenterRoleRequired
	}
	query.OfficeCode = access.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return ComplaintDetail{}, err
	}
	detail, err := service.repository.GetCompletedTicketForOffice(ctx, query)
	if errors.Is(err, sql.ErrNoRows) {
		return ComplaintDetail{}, ErrComplaintNotFoundOrNotVisible
	}
	return detail, err
}

func (service *Service) ListOpenTicketHistoryForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, query ComplaintHistoryQuery) ([]ComplaintStatusHistoryEntry, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	query.OfficeCode = ""
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}

	ticketAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	historyAccess, err := service.authorization.Authorize(ctx, identity, "cchentrimas", authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isCallCenter(ticketAccess.UserLevelIDs) || !isCallCenter(historyAccess.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	if ticketAccess.OfficeCode == "" || ticketAccess.OfficeCode != historyAccess.OfficeCode ||
		!sameRoleIDs(ticketAccess.UserLevelIDs, historyAccess.UserLevelIDs) {
		return nil, authorization.ErrIdentityMismatch
	}

	query.OfficeCode = ticketAccess.OfficeCode
	return service.repository.ListOpenTicketHistory(ctx, query)
}

func (service *Service) ListCompletedTicketHistoryForCallCenter(ctx context.Context, identity authentication.VerifiedIdentity, query ComplaintHistoryQuery) ([]ComplaintStatusHistoryEntry, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	query.OfficeCode = ""
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}
	access, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionView)
	if err != nil {
		return nil, err
	}
	if !isCallCenter(access.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	query.OfficeCode = access.OfficeCode
	return service.repository.ListCompletedTicketHistory(ctx, query)
}

func (service *Service) ListCallCenterStatusOptions(ctx context.Context, identity authentication.VerifiedIdentity, complaintID string) ([]ComplaintStatusOption, error) {
	if service == nil || service.authorization == nil || service.repository == nil {
		return nil, ErrComplaintServiceNotConfigured
	}
	query, err := (ComplaintDetailQuery{ComplaintID: complaintID}).NormalizeRequest()
	if err != nil {
		return nil, err
	}
	ticketAccess, err := service.authorization.Authorize(ctx, identity, cchComplaintTable, authorization.ActionView)
	if err != nil {
		return nil, err
	}
	statusAccess, err := service.authorization.Authorize(ctx, identity, "cchentrimas", authorization.ActionAdd)
	if err != nil {
		return nil, err
	}
	if !isCallCenter(ticketAccess.UserLevelIDs) || !isCallCenter(statusAccess.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	if ticketAccess.OfficeCode == "" || ticketAccess.OfficeCode != statusAccess.OfficeCode || !sameRoleIDs(ticketAccess.UserLevelIDs, statusAccess.UserLevelIDs) {
		return nil, authorization.ErrIdentityMismatch
	}
	options, err := service.repository.ListCallCenterStatusOptions(ctx, query.ComplaintID, ticketAccess.OfficeCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStatusOptionsNotAvailable
	}
	return options, err
}

func isCallCenter(userLevelIDs []int64) bool {
	return len(userLevelIDs) == 1 && userLevelIDs[0] == callCenterUserLevelID
}

func isOfficeDashboardRole(userLevelIDs []int64) bool {
	if len(userLevelIDs) != 1 {
		return false
	}
	switch userLevelIDs[0] {
	case 1, 2, 3, 9, 10, 19, 20, 29:
		return true
	default:
		return false
	}
}

func sameRoleIDs(left []int64, right []int64) bool {
	return slices.Equal(left, right)
}
