package complaints

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type stubComplaintAccessLoader struct {
	access        authorization.AccountAccess
	accessByTable map[string]authorization.AccountAccess
	err           error
	loadedTables  []string
}

func (loader *stubComplaintAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, tableVariable string) (authorization.AccountAccess, error) {
	loader.loadedTables = append(loader.loadedTables, tableVariable)
	if access, exists := loader.accessByTable[tableVariable]; exists {
		return access, loader.err
	}
	return loader.access, loader.err
}

type stubComplaintRepository struct {
	query                 ComplaintListQuery
	results               []ComplaintSummary
	err                   error
	called                bool
	historyQuery          ComplaintHistoryQuery
	history               []ComplaintStatusHistoryEntry
	historyErr            error
	historyCalled         bool
	detailQuery           ComplaintDetailQuery
	detail                ComplaintDetail
	detailErr             error
	detailCalled          bool
	statusID              string
	statusOffice          string
	statusOptions         []ComplaintStatusOption
	statusErr             error
	statusCalled          bool
	receivedReportQuery   ReceivedOpenReportQuery
	receivedReportOffice  string
	receivedReportRows    []ReceivedOpenReportRow
	receivedReportErr     error
	receivedReportCalled  bool
	sentReportQuery       SentOpenReportQuery
	sentReportOffice      string
	sentReportRows        []SentOpenReportRow
	sentReportErr         error
	sentReportCalled      bool
	durationQuery         ReceivedOpenDurationQuery
	durationOffice        string
	durationBuckets       ReceivedOpenDurationBuckets
	durationErr           error
	durationCalled        bool
	sentDurationQuery     SentOpenDurationQuery
	sentDurationOffice    string
	sentDurationBuckets   SentOpenDurationBuckets
	sentDurationErr       error
	sentDurationCalled    bool
	sentSummaryQuery      SentSummaryQuery
	sentSummaryOffice     string
	sentSummaryRows       []SentSummaryRow
	sentSummaryErr        error
	sentSummaryCalled     bool
	receivedSummaryQuery  ReceivedSummaryQuery
	receivedSummaryOffice string
	receivedSummaryRows   []ReceivedSummaryRow
	receivedSummaryErr    error
	receivedSummaryCalled bool
	breakdownQuery        BreakdownQuery
	breakdownOffice       string
	breakdownRows         []BreakdownRow
	breakdownErr          error
	breakdownCalled       bool
	countQuery            OpenCountQuery
	countOffice           string
	countMethod           string
}

func (repository *stubComplaintRepository) ListInvolvingOffice(_ context.Context, query ComplaintListQuery) ([]ComplaintSummary, error) {
	repository.called = true
	repository.query = query
	return repository.results, repository.err
}

func (repository *stubComplaintRepository) ListReceivedOpenReport(_ context.Context, query ReceivedOpenReportQuery, officeCode string) ([]ReceivedOpenReportRow, error) {
	repository.receivedReportCalled = true
	repository.receivedReportQuery = query
	repository.receivedReportOffice = officeCode
	return repository.receivedReportRows, repository.receivedReportErr
}

func (repository *stubComplaintRepository) ListSentOpenReport(_ context.Context, query SentOpenReportQuery, officeCode string) ([]SentOpenReportRow, error) {
	repository.sentReportCalled = true
	repository.sentReportQuery = query
	repository.sentReportOffice = officeCode
	return repository.sentReportRows, repository.sentReportErr
}

func (repository *stubComplaintRepository) CountReceivedOpenDurationBuckets(_ context.Context, query ReceivedOpenDurationQuery, officeCode string) (ReceivedOpenDurationBuckets, error) {
	repository.durationCalled = true
	repository.durationQuery = query
	repository.durationOffice = officeCode
	return repository.durationBuckets, repository.durationErr
}

func (repository *stubComplaintRepository) CountSentOpenDurationBuckets(_ context.Context, query SentOpenDurationQuery, officeCode string) (SentOpenDurationBuckets, error) {
	repository.sentDurationCalled = true
	repository.sentDurationQuery = query
	repository.sentDurationOffice = officeCode
	return repository.sentDurationBuckets, repository.sentDurationErr
}

func (repository *stubComplaintRepository) ListSentSummary(_ context.Context, query SentSummaryQuery, officeCode string) ([]SentSummaryRow, error) {
	repository.sentSummaryCalled = true
	repository.sentSummaryQuery = query
	repository.sentSummaryOffice = officeCode
	return repository.sentSummaryRows, repository.sentSummaryErr
}

func (repository *stubComplaintRepository) ListReceivedSummary(_ context.Context, query ReceivedSummaryQuery, officeCode string) ([]ReceivedSummaryRow, error) {
	repository.receivedSummaryCalled = true
	repository.receivedSummaryQuery = query
	repository.receivedSummaryOffice = officeCode
	return repository.receivedSummaryRows, repository.receivedSummaryErr
}

func (repository *stubComplaintRepository) ListBreakdown(_ context.Context, query BreakdownQuery, officeCode string) ([]BreakdownRow, error) {
	repository.breakdownCalled = true
	repository.breakdownQuery = query
	repository.breakdownOffice = officeCode
	return repository.breakdownRows, repository.breakdownErr
}

func (repository *stubComplaintRepository) ListOpenTicketHistory(_ context.Context, query ComplaintHistoryQuery) ([]ComplaintStatusHistoryEntry, error) {
	repository.historyCalled = true
	repository.historyQuery = query
	return repository.history, repository.historyErr
}

func (repository *stubComplaintRepository) ListCompletedTicketHistory(_ context.Context, query ComplaintHistoryQuery) ([]ComplaintStatusHistoryEntry, error) {
	repository.historyCalled = true
	repository.historyQuery = query
	return repository.history, repository.historyErr
}

func (repository *stubComplaintRepository) CountCompletedForCallCenter(_ context.Context, _ CompletedCountQuery, _ string) (int64, error) {
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountOpenForCallCenter(_ context.Context, _ OpenCountQuery, _ string) (int64, error) {
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountReceivedOpenForCallCenter(_ context.Context, query OpenCountQuery, officeCode string) (int64, error) {
	repository.countQuery = query
	repository.countOffice = officeCode
	repository.countMethod = "received"
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountSentOpenForCallCenter(_ context.Context, query OpenCountQuery, officeCode string) (int64, error) {
	repository.countQuery = query
	repository.countOffice = officeCode
	repository.countMethod = "sent"
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountDashboardOpen(_ context.Context, query OpenCountQuery, officeCode string) (int64, error) {
	repository.countQuery = query
	repository.countOffice = officeCode
	repository.countMethod = "dashboard-open"
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountDashboardReceivedOpen(_ context.Context, query OpenCountQuery, officeCode string) (int64, error) {
	repository.countQuery = query
	repository.countOffice = officeCode
	repository.countMethod = "dashboard-received"
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountDashboardSentOpen(_ context.Context, query OpenCountQuery, officeCode string) (int64, error) {
	repository.countQuery = query
	repository.countOffice = officeCode
	repository.countMethod = "dashboard-sent"
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) CountDashboardCompleted(_ context.Context, query CompletedCountQuery, officeCode string) (int64, error) {
	repository.countQuery = query
	repository.countOffice = officeCode
	repository.countMethod = "dashboard-completed"
	return int64(len(repository.results)), repository.err
}

func (repository *stubComplaintRepository) GetOpenTicketForOffice(_ context.Context, query ComplaintDetailQuery) (ComplaintDetail, error) {
	repository.detailCalled = true
	repository.detailQuery = query
	return repository.detail, repository.detailErr
}

func (repository *stubComplaintRepository) GetCompletedTicketForOffice(_ context.Context, query ComplaintDetailQuery) (ComplaintDetail, error) {
	repository.detailCalled = true
	repository.detailQuery = query
	return repository.detail, repository.detailErr
}

func (repository *stubComplaintRepository) ListCallCenterStatusOptions(_ context.Context, complaintID string, officeCode string) ([]ComplaintStatusOption, error) {
	repository.statusCalled = true
	repository.statusID = complaintID
	repository.statusOffice = officeCode
	return repository.statusOptions, repository.statusErr
}

func TestServiceListsOpenCallCenterTicketsUsingTrustedOffice(t *testing.T) {
	accessService := authorization.NewAccessService(&stubComplaintAccessLoader{
		access: authorization.AccountAccess{
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			UserLevelIDs:   []int64{callCenterUserLevelID},
			PermissionMask: int64(authorization.ActionList),
		},
	})
	repository := &stubComplaintRepository{}
	service := NewService(accessService, repository)
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}

	_, err := service.ListOpenForCallCenter(context.Background(), identity, ComplaintListQuery{
		OfficeCode: "ATTACKER",
		Limit:      15,
		Offset:     30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called {
		t.Fatal("complaint repository was not called")
	}
	if repository.query.OfficeCode != "40005CC" || repository.query.OfficeMatch != MatchOriginOrAllTargets || !repository.query.OpenOnly {
		t.Fatalf("repository query has untrusted or incomplete scope: %#v", repository.query)
	}
	if repository.query.Limit != 15 || repository.query.Offset != 30 {
		t.Fatalf("pagination was not preserved: %#v", repository.query)
	}
}

func TestServiceListsReceivedAndSentDashboardRowsWithTrustedOfficeAndScope(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name        string
		officeMatch ComplaintOfficeMatch
		list        func(*Service, ComplaintListQuery) ([]ComplaintSummary, error)
	}{
		{name: "received", officeMatch: MatchAllTargetsOnly, list: func(service *Service, query ComplaintListQuery) ([]ComplaintSummary, error) {
			return service.ListDashboardReceivedOpen(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, query)
		}},
		{name: "sent", officeMatch: MatchOriginOnly, list: func(service *Service, query ComplaintListQuery) ([]ComplaintSummary, error) {
			return service.ListDashboardSentOpen(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, query)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repository := newCallCenterTestService([]int64{10}, int64(authorization.ActionList), nil)
			_, err := test.list(service, ComplaintListQuery{Limit: 10, Offset: 20, From: &from, To: &to})
			if err != nil {
				t.Fatal(err)
			}
			query := repository.query
			if !repository.called || query.OfficeCode != "40005CC" || query.OfficeMatch != test.officeMatch || !query.OpenOnly || query.CompletedOnly {
				t.Fatalf("%s list query has untrusted or incorrect scope: %#v", test.name, query)
			}
			if query.Limit != 10 || query.Offset != 20 || query.From == nil || !query.From.Equal(from) || query.To == nil || !query.To.Equal(to) {
				t.Fatalf("%s list pagination/date filters lost: %#v", test.name, query)
			}
		})
	}
}

func TestServiceListsReceivedOpenReportWithThreeTableACL(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	loader := &stubComplaintAccessLoader{access: authorization.AccountAccess{
		Username: "regional.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10},
		PermissionMask: int64(authorization.ActionList),
	}}
	repository := &stubComplaintRepository{receivedReportRows: []ReceivedOpenReportRow{{ComplaintID: "TKT-001", LatestStatusCode: "102"}}}
	service := NewService(authorization.NewAccessService(loader), repository)
	rows, err := service.ListReceivedOpenReport(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, ReceivedOpenReportQuery{
		From: &from, To: &to, ProductCode: "REG", Limit: 10, Offset: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.receivedReportCalled || repository.receivedReportOffice != "40005CC" || repository.receivedReportQuery.ProductCode != "REG" || repository.receivedReportQuery.Limit != 10 || repository.receivedReportQuery.Offset != 20 {
		t.Fatalf("report repository received unexpected access/query: %#v", repository)
	}
	if len(rows) != 1 || rows[0].ComplaintID != "TKT-001" {
		t.Fatalf("report rows = %#v", rows)
	}
	if len(loader.loadedTables) != 3 || loader.loadedTables[0] != "cchentri" || loader.loadedTables[1] != "cchentridet" || loader.loadedTables[2] != "cchentrimas" {
		t.Fatalf("authorized tables = %#v, want parent/distribution/history", loader.loadedTables)
	}
}

func TestServiceRejectsReceivedOpenReportOfficeMismatchAndMissingHistoryPermission(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name     string
		accessBy map[string]authorization.AccountAccess
		want     error
	}{
		{
			name: "office mismatch",
			accessBy: map[string]authorization.AccountAccess{
				"cchentridet": {OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionList)},
			},
			want: authorization.ErrIdentityMismatch,
		},
		{
			name: "missing history list permission",
			accessBy: map[string]authorization.AccountAccess{
				"cchentrimas": {OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionView)},
			},
			want: authorization.ErrPermissionDenied,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionList)}
			loader := &stubComplaintAccessLoader{access: base, accessByTable: test.accessBy}
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListReceivedOpenReport(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ReceivedOpenReportQuery{From: &from, To: &to})
			if !errors.Is(err, test.want) {
				t.Fatalf("ListReceivedOpenReport() error = %v, want %v", err, test.want)
			}
			if repository.receivedReportCalled {
				t.Fatal("denied report request reached repository")
			}
		})
	}
}

func TestServiceListsSentOpenReportWithThreeTableACL(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	loader := &stubComplaintAccessLoader{access: authorization.AccountAccess{
		Username: "regional.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10},
		PermissionMask: int64(authorization.ActionList),
	}}
	repository := &stubComplaintRepository{sentReportRows: []SentOpenReportRow{{ComplaintID: "TKT-001", LatestStatusCode: "102"}}}
	service := NewService(authorization.NewAccessService(loader), repository)
	rows, err := service.ListSentOpenReport(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, SentOpenReportQuery{
		From: &from, To: &to, ProductCode: "REG", Limit: 10, Offset: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.sentReportCalled || repository.sentReportOffice != "40005CC" || repository.sentReportQuery.ProductCode != "REG" || repository.sentReportQuery.Limit != 10 || repository.sentReportQuery.Offset != 20 {
		t.Fatalf("report repository received unexpected access/query: %#v", repository)
	}
	if len(rows) != 1 || rows[0].ComplaintID != "TKT-001" {
		t.Fatalf("report rows = %#v", rows)
	}
	if len(loader.loadedTables) != 3 || loader.loadedTables[0] != "cchentri" || loader.loadedTables[1] != "cchentridet" || loader.loadedTables[2] != "cchentrimas" {
		t.Fatalf("authorized tables = %#v, want parent/distribution/history", loader.loadedTables)
	}
}

func TestServiceRejectsSentOpenReportOfficeMismatchAndMissingHistoryPermission(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name     string
		accessBy map[string]authorization.AccountAccess
		want     error
	}{
		{
			name: "office mismatch",
			accessBy: map[string]authorization.AccountAccess{
				"cchentridet": {OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionList)},
			},
			want: authorization.ErrIdentityMismatch,
		},
		{
			name: "missing history list permission",
			accessBy: map[string]authorization.AccountAccess{
				"cchentrimas": {OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionView)},
			},
			want: authorization.ErrPermissionDenied,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionList)}
			loader := &stubComplaintAccessLoader{access: base, accessByTable: test.accessBy}
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListSentOpenReport(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, SentOpenReportQuery{From: &from, To: &to})
			if !errors.Is(err, test.want) {
				t.Fatalf("ListSentOpenReport() error = %v, want %v", err, test.want)
			}
			if repository.sentReportCalled {
				t.Fatal("denied report request reached repository")
			}
		})
	}
}

func TestServiceCountsReceivedOpenDurationBucketsWithParentAndDistributionACL(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
	loader := &stubComplaintAccessLoader{access: base}
	want := ReceivedOpenDurationBuckets{DayZero: 1, DayOne: 2, DayTwo: 3, DayThree: 4, DayFourPlus: 5}
	repository := &stubComplaintRepository{durationBuckets: want}
	service := NewService(authorization.NewAccessService(loader), repository)
	got, err := service.CountReceivedOpenDurationBuckets(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, ReceivedOpenDurationQuery{
		From: &from, To: &to, ProductCode: "REG",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want || !repository.durationCalled || repository.durationOffice != "40005CC" || repository.durationQuery.ProductCode != "REG" {
		t.Fatalf("duration result/scope = %#v/%#v", got, repository)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != "cchentri" || loader.loadedTables[1] != "cchentridet" {
		t.Fatalf("authorized tables = %#v, want parent and distribution", loader.loadedTables)
	}
}

func TestServiceRejectsReceivedOpenDurationDistributionPermissionOrOfficeMismatch(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name   string
		access authorization.AccountAccess
		want   error
	}{
		{name: "missing distribution List", access: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionView)}, want: authorization.ErrPermissionDenied},
		{name: "office mismatch", access: authorization.AccountAccess{OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}, want: authorization.ErrIdentityMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
			loader := &stubComplaintAccessLoader{access: base, accessByTable: map[string]authorization.AccountAccess{"cchentridet": test.access}}
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.CountReceivedOpenDurationBuckets(context.Background(), authentication.VerifiedIdentity{Username: "regional.test"}, ReceivedOpenDurationQuery{From: &from, To: &to})
			if !errors.Is(err, test.want) {
				t.Fatalf("CountReceivedOpenDurationBuckets() error = %v, want %v", err, test.want)
			}
			if repository.durationCalled {
				t.Fatal("denied duration report reached repository")
			}
		})
	}
}

func TestServiceCountsSentOpenDurationBucketsWithParentDistributionAndHistoryACL(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
	loader := &stubComplaintAccessLoader{access: base}
	want := SentOpenDurationBuckets{DayZero: 1, DayOne: 2, DayTwo: 3, DayThree: 4, DayFourPlus: 5}
	repository := &stubComplaintRepository{sentDurationBuckets: want}
	service := NewService(authorization.NewAccessService(loader), repository)
	got, err := service.CountSentOpenDurationBuckets(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, SentOpenDurationQuery{
		From: &from, To: &to, ProductCode: "REG",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want || !repository.sentDurationCalled || repository.sentDurationOffice != "40005CC" || repository.sentDurationQuery.ProductCode != "REG" {
		t.Fatalf("duration result/scope = %#v/%#v", got, repository)
	}
	if len(loader.loadedTables) != 3 || loader.loadedTables[0] != "cchentri" || loader.loadedTables[1] != "cchentridet" || loader.loadedTables[2] != "cchentrimas" {
		t.Fatalf("authorized tables = %#v, want parent, distribution, and history", loader.loadedTables)
	}
}

func TestServiceRejectsSentOpenDurationHistoryPermissionOrOfficeMismatch(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name   string
		access authorization.AccountAccess
		want   error
	}{
		{name: "missing history List", access: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionView)}, want: authorization.ErrPermissionDenied},
		{name: "office mismatch", access: authorization.AccountAccess{OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}, want: authorization.ErrIdentityMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
			loader := &stubComplaintAccessLoader{access: base, accessByTable: map[string]authorization.AccountAccess{"cchentrimas": test.access}}
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.CountSentOpenDurationBuckets(context.Background(), authentication.VerifiedIdentity{Username: "regional.test"}, SentOpenDurationQuery{From: &from, To: &to})
			if !errors.Is(err, test.want) {
				t.Fatalf("CountSentOpenDurationBuckets() error = %v, want %v", err, test.want)
			}
			if repository.sentDurationCalled {
				t.Fatal("denied duration report reached repository")
			}
		})
	}
}

func TestServiceListsSentSummaryWithParentAndOfficeACL(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
	loader := &stubComplaintAccessLoader{access: base}
	want := []SentSummaryRow{{OfficeCode: "40005CC", OfficeName: "Divre A", Total: 10, Completed: 6, Open: 4, Responded: 7, Unresponded: 3}}
	repository := &stubComplaintRepository{sentSummaryRows: want}
	service := NewService(authorization.NewAccessService(loader), repository)
	got, err := service.ListSentSummary(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, SentSummaryQuery{
		From: &from, To: &to, ProductCode: "REG", RegionalCode: "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] || !repository.sentSummaryCalled || repository.sentSummaryOffice != "40005CC" || repository.sentSummaryQuery.RegionalCode != "" {
		t.Fatalf("summary result/scope = %#v/%#v", got, repository)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != "cchentri" || loader.loadedTables[1] != "cch_kantor" {
		t.Fatalf("authorized tables = %#v, want parent and office", loader.loadedTables)
	}
}

func TestServiceRejectsSentSummaryOfficePermissionOrMismatch(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name   string
		access authorization.AccountAccess
		want   error
	}{
		{name: "missing office List", access: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionView)}, want: authorization.ErrPermissionDenied},
		{name: "office mismatch", access: authorization.AccountAccess{OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}, want: authorization.ErrIdentityMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
			loader := &stubComplaintAccessLoader{access: base, accessByTable: map[string]authorization.AccountAccess{"cch_kantor": test.access}}
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListSentSummary(context.Background(), authentication.VerifiedIdentity{Username: "regional.test"}, SentSummaryQuery{From: &from, To: &to})
			if !errors.Is(err, test.want) {
				t.Fatalf("ListSentSummary() error = %v, want %v", err, test.want)
			}
			if repository.sentSummaryCalled {
				t.Fatal("denied summary reached repository")
			}
		})
	}
}

func TestServiceListsReceivedSummaryWithParentDistributionAndOfficeACL(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
	loader := &stubComplaintAccessLoader{access: base}
	want := []ReceivedSummaryRow{{OfficeCode: "40006CC", OfficeName: "Divre B", Total: 10, Completed: 6, Open: 4, Responded: 7, Unresponded: 3}}
	repository := &stubComplaintRepository{receivedSummaryRows: want}
	service := NewService(authorization.NewAccessService(loader), repository)
	got, err := service.ListReceivedSummary(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, ReceivedSummaryQuery{
		From: &from, To: &to, ProductCode: "REG", RegionalCode: "0", OnlyTarget: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] || !repository.receivedSummaryCalled || repository.receivedSummaryOffice != "40005CC" || repository.receivedSummaryQuery.RegionalCode != "" || !repository.receivedSummaryQuery.OnlyTarget {
		t.Fatalf("summary result/scope = %#v/%#v", got, repository)
	}
	if len(loader.loadedTables) != 3 || loader.loadedTables[0] != "cchentri" || loader.loadedTables[1] != "cchentridet" || loader.loadedTables[2] != "cch_kantor" {
		t.Fatalf("authorized tables = %#v, want parent, distribution, and office", loader.loadedTables)
	}
}

func TestServiceRejectsReceivedSummaryDistributionPermissionOrMismatch(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name   string
		access authorization.AccountAccess
		want   error
	}{
		{name: "missing distribution List", access: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionView)}, want: authorization.ErrPermissionDenied},
		{name: "office mismatch", access: authorization.AccountAccess{OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}, want: authorization.ErrIdentityMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
			loader := &stubComplaintAccessLoader{access: base, accessByTable: map[string]authorization.AccountAccess{"cchentridet": test.access}}
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListReceivedSummary(context.Background(), authentication.VerifiedIdentity{Username: "regional.test"}, ReceivedSummaryQuery{From: &from, To: &to})
			if !errors.Is(err, test.want) {
				t.Fatalf("ListReceivedSummary() error = %v, want %v", err, test.want)
			}
			if repository.receivedSummaryCalled {
				t.Fatal("denied summary reached repository")
			}
		})
	}
}

func TestServiceListsBreakdownForSentAndReceivedWithCorrectTables(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
	want := []BreakdownRow{{Code: "REG", Label: "Reguler", Count: 7}}
	for _, test := range []struct {
		name       string
		direction  BreakdownDirection
		wantTables []string
	}{
		{name: "sent", direction: BreakdownDirectionSent, wantTables: []string{"cchentri", "cch_kantor"}},
		{name: "received", direction: BreakdownDirectionReceived, wantTables: []string{"cchentri", "cch_kantor", "cchentridet"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &stubComplaintAccessLoader{access: base}
			repository := &stubComplaintRepository{breakdownRows: want}
			service := NewService(authorization.NewAccessService(loader), repository)
			got, err := service.ListBreakdown(context.Background(), authentication.VerifiedIdentity{Username: "regional.test", OfficeCode: "ATTACKER"}, BreakdownQuery{
				Direction: test.direction, Kind: BreakdownKindShipment, From: &from, To: &to, RegionalCode: "0",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0] != want[0] || !repository.breakdownCalled || repository.breakdownOffice != "40005CC" || repository.breakdownQuery.RegionalCode != "" {
				t.Fatalf("breakdown result/scope = %#v/%#v", got, repository)
			}
			if len(loader.loadedTables) != len(test.wantTables) {
				t.Fatalf("authorized tables = %#v, want %#v", loader.loadedTables, test.wantTables)
			}
			for index, table := range test.wantTables {
				if loader.loadedTables[index] != table {
					t.Fatalf("authorized tables = %#v, want %#v", loader.loadedTables, test.wantTables)
				}
			}
		})
	}
}

func TestServiceRejectsBreakdownOnMissingDistributionPermissionForReceived(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	base := authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionList)}
	loader := &stubComplaintAccessLoader{access: base, accessByTable: map[string]authorization.AccountAccess{"cchentridet": {
		OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionView),
	}}}
	repository := &stubComplaintRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListBreakdown(context.Background(), authentication.VerifiedIdentity{Username: "regional.test"}, BreakdownQuery{
		Direction: BreakdownDirectionReceived, Kind: BreakdownKindProblem, From: &from, To: &to,
	})
	if !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("ListBreakdown() error = %v, want permission denied", err)
	}
	if repository.breakdownCalled {
		t.Fatal("denied breakdown reached repository")
	}
}

func TestServiceGetsOpenTicketDetailUsingTrustedOffice(t *testing.T) {
	loader := &stubComplaintAccessLoader{access: authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterUserLevelID},
		PermissionMask: int64(authorization.ActionView),
	}}
	repository := &stubComplaintRepository{detail: ComplaintDetail{ComplaintID: "TKT-001"}}
	service := NewService(authorization.NewAccessService(loader), repository)

	detail, err := service.GetOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, " TKT-001 ")
	if err != nil {
		t.Fatal(err)
	}
	if !repository.detailCalled || repository.detailQuery.ComplaintID != "TKT-001" || repository.detailQuery.OfficeCode != "40005CC" {
		t.Fatalf("detail query has untrusted or incomplete scope: %#v", repository.detailQuery)
	}
	if detail.ComplaintID != "TKT-001" {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestServiceRequiresViewPermissionForComplaintDetail(t *testing.T) {
	service, repository := newCallCenterTestService([]int64{callCenterUserLevelID}, int64(authorization.ActionList), nil)
	_, err := service.GetOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, "TKT-001")
	if !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("GetOpenForCallCenter() error = %v, want View permission denial", err)
	}
	if repository.detailCalled {
		t.Fatal("permission-denied request reached complaint detail repository")
	}
}

func TestServiceListsOnlyLegacyCallCenterOriginStatusOptions(t *testing.T) {
	base := authorization.AccountAccess{
		OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterUserLevelID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionAdd),
	}
	loader := &stubComplaintAccessLoader{access: base}
	repository := &stubComplaintRepository{statusOptions: []ComplaintStatusOption{{Code: "102", Description: "Respon"}}}
	service := NewService(authorization.NewAccessService(loader), repository)
	options, err := service.ListCallCenterStatusOptions(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, " TKT-001 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 1 || options[0].Code != "102" || !repository.statusCalled || repository.statusOffice != "40005CC" {
		t.Fatalf("status options result/scope = %#v/%#v", options, repository)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != cchComplaintTable || loader.loadedTables[1] != "cchentrimas" {
		t.Fatalf("authorized tables = %#v, want cchentri and cchentrimas", loader.loadedTables)
	}
}

func TestServiceDeniesStatusOptionsWithoutAddRoleOrMatchingOffice(t *testing.T) {
	for _, test := range []struct {
		name    string
		access  map[string]authorization.AccountAccess
		wantErr error
	}{
		{
			name: "no history Add permission",
			access: map[string]authorization.AccountAccess{
				cchComplaintTable: {OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterUserLevelID}, PermissionMask: int64(authorization.ActionView)},
				"cchentrimas":     {OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterUserLevelID}, PermissionMask: int64(authorization.ActionList)},
			},
			wantErr: authorization.ErrPermissionDenied,
		},
		{
			name: "wrong role",
			access: map[string]authorization.AccountAccess{
				cchComplaintTable: {OfficeCode: "40005CC", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionView)},
				"cchentrimas":     {OfficeCode: "40005CC", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionAdd)},
			},
			wantErr: ErrCallCenterRoleRequired,
		},
		{
			name: "office mismatch",
			access: map[string]authorization.AccountAccess{
				cchComplaintTable: {OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterUserLevelID}, PermissionMask: int64(authorization.ActionView)},
				"cchentrimas":     {OfficeCode: "40006CC", UserLevelIDs: []int64{callCenterUserLevelID}, PermissionMask: int64(authorization.ActionAdd)},
			},
			wantErr: authorization.ErrIdentityMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubComplaintRepository{}
			service := NewService(authorization.NewAccessService(&stubComplaintAccessLoader{accessByTable: test.access}), repository)
			if _, err := service.ListCallCenterStatusOptions(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, "TKT-001"); !errors.Is(err, test.wantErr) {
				t.Fatalf("status options error = %v, want %v", err, test.wantErr)
			}
			if repository.statusCalled {
				t.Fatal("repository called after status option authorization failure")
			}
		})
	}
}

func TestServiceHidesMissingAndOutOfScopeComplaintDetails(t *testing.T) {
	service, repository := newCallCenterTestService([]int64{callCenterUserLevelID}, int64(authorization.ActionView), nil)
	repository.detailErr = sql.ErrNoRows
	_, err := service.GetOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, "TKT-UNKNOWN")
	if !errors.Is(err, ErrComplaintNotFoundOrNotVisible) {
		t.Fatalf("GetOpenForCallCenter() error = %v, want not found or not visible", err)
	}
}

func TestServiceRejectsNonCallCenterAndMultiRoleAccounts(t *testing.T) {
	for _, test := range []struct {
		name  string
		roles []int64
	}{
		{name: "other role", roles: []int64{1}},
		{name: "multiple roles", roles: []int64{callCenterUserLevelID, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repository := newCallCenterTestService(test.roles, int64(authorization.ActionList), nil)
			_, err := service.ListOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintListQuery{})
			if !errors.Is(err, ErrCallCenterRoleRequired) {
				t.Fatalf("ListOpenForCallCenter() error = %v, want Call Center role denial", err)
			}
			if repository.called {
				t.Fatal("unsupported role reached complaint repository")
			}
		})
	}
}

func TestServiceRequiresListPermissionBeforeQuery(t *testing.T) {
	service, repository := newCallCenterTestService([]int64{callCenterUserLevelID}, int64(authorization.ActionView), nil)
	_, err := service.ListOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintListQuery{})
	if !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("ListOpenForCallCenter() error = %v, want permission denied", err)
	}
	if repository.called {
		t.Fatal("permission-denied request reached complaint repository")
	}
}

func TestServiceCountsReceivedAndSentOpenRequestsUsingTrustedOffice(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name  string
		count func(*Service, OpenCountQuery) (int64, error)
		kind  string
	}{
		{
			name: "received",
			kind: "received",
			count: func(service *Service, query OpenCountQuery) (int64, error) {
				return service.CountReceivedOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, query)
			},
		},
		{
			name: "sent",
			kind: "sent",
			count: func(service *Service, query OpenCountQuery) (int64, error) {
				return service.CountSentOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, query)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repository := newCallCenterTestService([]int64{callCenterUserLevelID}, int64(authorization.ActionList), nil)
			count, err := test.count(service, OpenCountQuery{From: &from, To: &to})
			if err != nil || count != 0 {
				t.Fatalf("count = %d, error = %v", count, err)
			}
			if repository.countMethod != test.kind || repository.countOffice != "40005CC" || repository.countQuery.From != &from || repository.countQuery.To != &to {
				t.Fatalf("repository received wrong count scope: %#v", repository)
			}
		})
	}
}

func TestServiceExportsOpenCallCenterComplaintsWithListAndExportPermission(t *testing.T) {
	loader := &stubComplaintAccessLoader{access: authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterUserLevelID},
		PermissionMask: int64(authorization.ActionList | authorization.ActionExport),
	}}
	repository := &stubComplaintRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}

	_, err := service.ExportOpenForCallCenter(context.Background(), identity, ComplaintListQuery{
		OfficeCode: "ATTACKER",
		Limit:      10,
		Offset:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || repository.query.OfficeCode != "40005CC" || repository.query.OfficeMatch != MatchOriginOrAllTargets || !repository.query.OpenOnly || repository.query.CompletedOnly {
		t.Fatalf("export query has untrusted or incorrect scope: %#v", repository.query)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != cchComplaintTable || loader.loadedTables[1] != cchComplaintTable {
		t.Fatalf("export authorized tables = %#v, want List and Export checks on cchentri", loader.loadedTables)
	}
}

func TestServiceRejectsOpenComplaintExportWithoutExportPermission(t *testing.T) {
	service, repository := newCallCenterTestService([]int64{callCenterUserLevelID}, int64(authorization.ActionList), nil)
	_, err := service.ExportOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintListQuery{})
	if !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("ExportOpenForCallCenter() error = %v, want Export permission denial", err)
	}
	if repository.called {
		t.Fatal("export repository was called without Export permission")
	}
}

func TestServiceRequiresCallCenterListPermissionForReceivedAndSentCounts(t *testing.T) {
	for _, test := range []struct {
		name  string
		roles []int64
		mask  int64
		want  error
	}{
		{name: "non call center", roles: []int64{1}, mask: int64(authorization.ActionList), want: ErrCallCenterRoleRequired},
		{name: "multiple roles", roles: []int64{callCenterUserLevelID, 1}, mask: int64(authorization.ActionList), want: ErrCallCenterRoleRequired},
		{name: "missing list", roles: []int64{callCenterUserLevelID}, mask: int64(authorization.ActionView), want: authorization.ErrPermissionDenied},
	} {
		for _, count := range []struct {
			name string
			call func(*Service) (int64, error)
		}{
			{name: "received", call: func(service *Service) (int64, error) {
				return service.CountReceivedOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, OpenCountQuery{})
			}},
			{name: "sent", call: func(service *Service) (int64, error) {
				return service.CountSentOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, OpenCountQuery{})
			}},
		} {
			t.Run(test.name+"/"+count.name, func(t *testing.T) {
				service, repository := newCallCenterTestService(test.roles, test.mask, nil)
				_, err := count.call(service)
				if !errors.Is(err, test.want) {
					t.Fatalf("count error = %v, want %v", err, test.want)
				}
				if repository.countMethod != "" {
					t.Fatal("denied dashboard request reached repository")
				}
			})
		}
	}
}

func TestServiceAllowsOfficeDashboardRolesAndRejectsUnsupportedRoles(t *testing.T) {
	for _, roleID := range []int64{1, 2, 3, 9, 10, 19, 20, 29} {
		t.Run(strconv.FormatInt(roleID, 10), func(t *testing.T) {
			service, repository := newCallCenterTestService([]int64{roleID}, int64(authorization.ActionList), nil)
			_, err := service.CountDashboardReceivedOpen(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}, OpenCountQuery{})
			if err != nil {
				t.Fatalf("CountDashboardReceivedOpen() error = %v", err)
			}
			if repository.countOffice != "40005CC" || repository.countMethod != "received" {
				t.Fatalf("dashboard used untrusted or unexpected scope: %#v", repository)
			}
		})
	}
	for _, roles := range [][]int64{{-1}, {99}, {3, 1}} {
		service, repository := newCallCenterTestService(roles, int64(authorization.ActionList), nil)
		_, err := service.CountDashboardReceivedOpen(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, OpenCountQuery{})
		if !errors.Is(err, ErrDashboardRoleRequired) {
			t.Errorf("roles %v error = %v, want unsupported role denial", roles, err)
		}
		if repository.countMethod != "" {
			t.Errorf("roles %v reached dashboard repository", roles)
		}
	}
}

func TestServicePropagatesRepositoryFailure(t *testing.T) {
	wantErr := errors.New("database unavailable")
	service, _ := newCallCenterTestService([]int64{callCenterUserLevelID}, int64(authorization.ActionList), wantErr)
	_, err := service.ListOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintListQuery{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListOpenForCallCenter() error = %v, want repository error", err)
	}
}

func TestServiceRequiresDependencies(t *testing.T) {
	var service *Service
	_, err := service.ListOpenForCallCenter(context.Background(), authentication.VerifiedIdentity{}, ComplaintListQuery{})
	if !errors.Is(err, ErrComplaintServiceNotConfigured) {
		t.Fatalf("ListOpenForCallCenter() error = %v, want service not configured", err)
	}
}

func TestServiceListsHistoryWithBothTablePermissionsAndTrustedOffice(t *testing.T) {
	loader := &stubComplaintAccessLoader{access: authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterUserLevelID},
		PermissionMask: int64(authorization.ActionList),
	}}
	repository := &stubComplaintRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)

	_, err := service.ListOpenTicketHistoryForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintHistoryQuery{
		ComplaintID: "  TKT-001 ",
		Limit:       10,
		Offset:      20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.historyCalled || repository.historyQuery.ComplaintID != "TKT-001" || repository.historyQuery.OfficeCode != "40005CC" {
		t.Fatalf("history query has untrusted or missing scope: %#v", repository.historyQuery)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != cchComplaintTable || loader.loadedTables[1] != "cchentrimas" {
		t.Fatalf("authorized tables = %#v, want cchentri and cchentrimas", loader.loadedTables)
	}
	if repository.historyQuery.Limit != 10 || repository.historyQuery.Offset != 20 {
		t.Fatalf("history pagination = %#v", repository.historyQuery)
	}
}

func TestServiceRequiresHistoryTableListPermission(t *testing.T) {
	loader := &stubComplaintAccessLoader{
		access: authorization.AccountAccess{
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			UserLevelIDs:   []int64{callCenterUserLevelID},
			PermissionMask: int64(authorization.ActionList),
		},
		accessByTable: map[string]authorization.AccountAccess{
			"cchentrimas": {
				Username:       "agent.test",
				OfficeCode:     "40005CC",
				UserLevelIDs:   []int64{callCenterUserLevelID},
				PermissionMask: int64(authorization.ActionView),
			},
		},
	}
	repository := &stubComplaintRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListOpenTicketHistoryForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintHistoryQuery{ComplaintID: "TKT-001"})
	if !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("ListOpenTicketHistoryForCallCenter() error = %v, want history List permission denial", err)
	}
	if repository.historyCalled {
		t.Fatal("history repository was called without cchentrimas List permission")
	}
}

func TestServiceRejectsOfficeChangeBetweenTicketAndHistoryAuthorization(t *testing.T) {
	loader := &stubComplaintAccessLoader{
		access: authorization.AccountAccess{
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			UserLevelIDs:   []int64{callCenterUserLevelID},
			PermissionMask: int64(authorization.ActionList),
		},
		accessByTable: map[string]authorization.AccountAccess{
			"cchentrimas": {
				Username:       "agent.test",
				OfficeCode:     "40006CC",
				UserLevelIDs:   []int64{callCenterUserLevelID},
				PermissionMask: int64(authorization.ActionList),
			},
		},
	}
	repository := &stubComplaintRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListOpenTicketHistoryForCallCenter(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, ComplaintHistoryQuery{ComplaintID: "TKT-001"})
	if !errors.Is(err, authorization.ErrIdentityMismatch) {
		t.Fatalf("ListOpenTicketHistoryForCallCenter() error = %v, want identity mismatch", err)
	}
	if repository.historyCalled {
		t.Fatal("office mismatch reached history repository")
	}
}

func TestServiceRejectsInvalidHistoryQueryBeforeAuthorization(t *testing.T) {
	loader := &stubComplaintAccessLoader{}
	service := NewService(authorization.NewAccessService(loader), &stubComplaintRepository{})
	_, err := service.ListOpenTicketHistoryForCallCenter(context.Background(), authentication.VerifiedIdentity{}, ComplaintHistoryQuery{ComplaintID: ""})
	if !errors.Is(err, ErrInvalidComplaintHistoryQuery) {
		t.Fatalf("ListOpenTicketHistoryForCallCenter() error = %v, want invalid history query", err)
	}
	if len(loader.loadedTables) != 0 {
		t.Fatal("invalid history query reached authorization loader")
	}
}

func newCallCenterTestService(roles []int64, mask int64, repositoryErr error) (*Service, *stubComplaintRepository) {
	accessService := authorization.NewAccessService(&stubComplaintAccessLoader{
		access: authorization.AccountAccess{
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			UserLevelIDs:   roles,
			PermissionMask: mask,
		},
	})
	repository := &stubComplaintRepository{err: repositoryErr}
	return NewService(accessService, repository), repository
}
