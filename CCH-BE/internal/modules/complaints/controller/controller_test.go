package controller

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/complaints"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubOpenCallCenterLister struct {
	identity              authentication.VerifiedIdentity
	query                 complaints.ComplaintListQuery
	results               []complaints.ComplaintSummary
	historyQuery          complaints.ComplaintHistoryQuery
	historyEntries        []complaints.ComplaintStatusHistoryEntry
	detailID              string
	detail                complaints.ComplaintDetail
	statusOptions         []complaints.ComplaintStatusOption
	statusID              string
	receivedReportQuery   complaints.ReceivedOpenReportQuery
	receivedReportRows    []complaints.ReceivedOpenReportRow
	receivedReportCalled  bool
	sentReportQuery       complaints.SentOpenReportQuery
	sentReportRows        []complaints.SentOpenReportRow
	sentReportCalled      bool
	durationQuery         complaints.ReceivedOpenDurationQuery
	durationBuckets       complaints.ReceivedOpenDurationBuckets
	durationCalled        bool
	sentDurationQuery     complaints.SentOpenDurationQuery
	sentDurationBuckets   complaints.SentOpenDurationBuckets
	sentDurationCalled    bool
	sentSummaryQuery      complaints.SentSummaryQuery
	sentSummaryRows       []complaints.SentSummaryRow
	sentSummaryCalled     bool
	receivedSummaryQuery  complaints.ReceivedSummaryQuery
	receivedSummaryRows   []complaints.ReceivedSummaryRow
	receivedSummaryCalled bool
	breakdownQuery        complaints.BreakdownQuery
	breakdownRows         []complaints.BreakdownRow
	breakdownCalled       bool
	err                   error
	called                bool
	exportCalled          bool
	historyCalled         bool
	detailCalled          bool
}

func (service *stubOpenCallCenterLister) ListOpenForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ComplaintListQuery) ([]complaints.ComplaintSummary, error) {
	service.called = true
	service.identity = identity
	service.query = query
	return service.results, service.err
}

func (service *stubOpenCallCenterLister) ListDashboardReceivedOpen(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ComplaintListQuery) ([]complaints.ComplaintSummary, error) {
	service.called = true
	service.identity = identity
	service.query = query
	return service.results, service.err
}

func (service *stubOpenCallCenterLister) ListDashboardSentOpen(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ComplaintListQuery) ([]complaints.ComplaintSummary, error) {
	service.called = true
	service.identity = identity
	service.query = query
	return service.results, service.err
}

func (service *stubOpenCallCenterLister) ListReceivedOpenReport(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ReceivedOpenReportQuery) ([]complaints.ReceivedOpenReportRow, error) {
	service.receivedReportCalled = true
	service.identity = identity
	service.receivedReportQuery = query
	return service.receivedReportRows, service.err
}

func (service *stubOpenCallCenterLister) ListSentOpenReport(_ context.Context, identity authentication.VerifiedIdentity, query complaints.SentOpenReportQuery) ([]complaints.SentOpenReportRow, error) {
	service.sentReportCalled = true
	service.identity = identity
	service.sentReportQuery = query
	return service.sentReportRows, service.err
}

func (service *stubOpenCallCenterLister) CountReceivedOpenDurationBuckets(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ReceivedOpenDurationQuery) (complaints.ReceivedOpenDurationBuckets, error) {
	service.durationCalled = true
	service.identity = identity
	service.durationQuery = query
	return service.durationBuckets, service.err
}

func (service *stubOpenCallCenterLister) CountSentOpenDurationBuckets(_ context.Context, identity authentication.VerifiedIdentity, query complaints.SentOpenDurationQuery) (complaints.SentOpenDurationBuckets, error) {
	service.sentDurationCalled = true
	service.identity = identity
	service.sentDurationQuery = query
	return service.sentDurationBuckets, service.err
}

func (service *stubOpenCallCenterLister) ListSentSummary(_ context.Context, identity authentication.VerifiedIdentity, query complaints.SentSummaryQuery) ([]complaints.SentSummaryRow, error) {
	service.sentSummaryCalled = true
	service.identity = identity
	service.sentSummaryQuery = query
	return service.sentSummaryRows, service.err
}

func (service *stubOpenCallCenterLister) ListReceivedSummary(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ReceivedSummaryQuery) ([]complaints.ReceivedSummaryRow, error) {
	service.receivedSummaryCalled = true
	service.identity = identity
	service.receivedSummaryQuery = query
	return service.receivedSummaryRows, service.err
}

func (service *stubOpenCallCenterLister) ListBreakdown(_ context.Context, identity authentication.VerifiedIdentity, query complaints.BreakdownQuery) ([]complaints.BreakdownRow, error) {
	service.breakdownCalled = true
	service.identity = identity
	service.breakdownQuery = query
	return service.breakdownRows, service.err
}

func (service *stubOpenCallCenterLister) ExportOpenForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ComplaintListQuery) ([]complaints.ComplaintSummary, error) {
	service.exportCalled = true
	service.identity = identity
	service.query = query
	return service.results, service.err
}

func (service *stubOpenCallCenterLister) ListCompletedForCallCenter(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.ComplaintListQuery) ([]complaints.ComplaintSummary, error) {
	return service.results, service.err
}

func (service *stubOpenCallCenterLister) CountCompletedForCallCenter(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.CompletedCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountOpenForCallCenter(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.OpenCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountReceivedOpenForCallCenter(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.OpenCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountSentOpenForCallCenter(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.OpenCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountDashboardOpen(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.OpenCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountDashboardReceivedOpen(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.OpenCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountDashboardSentOpen(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.OpenCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) CountDashboardCompleted(_ context.Context, _ authentication.VerifiedIdentity, _ complaints.CompletedCountQuery) (int64, error) {
	return int64(len(service.results)), service.err
}

func (service *stubOpenCallCenterLister) ListOpenTicketHistoryForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ComplaintHistoryQuery) ([]complaints.ComplaintStatusHistoryEntry, error) {
	service.historyCalled = true
	service.identity = identity
	service.historyQuery = query
	return service.historyEntries, service.err
}

func (service *stubOpenCallCenterLister) ListCompletedTicketHistoryForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, query complaints.ComplaintHistoryQuery) ([]complaints.ComplaintStatusHistoryEntry, error) {
	service.historyCalled = true
	service.identity = identity
	service.historyQuery = query
	return service.historyEntries, service.err
}

func (service *stubOpenCallCenterLister) GetOpenForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, complaintID string) (complaints.ComplaintDetail, error) {
	service.detailCalled = true
	service.identity = identity
	service.detailID = complaintID
	return service.detail, service.err
}

func (service *stubOpenCallCenterLister) GetCompletedForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, complaintID string) (complaints.ComplaintDetail, error) {
	service.detailCalled = true
	service.identity = identity
	service.detailID = complaintID
	return service.detail, service.err
}

func (service *stubOpenCallCenterLister) ListCallCenterStatusOptions(_ context.Context, identity authentication.VerifiedIdentity, complaintID string) ([]complaints.ComplaintStatusOption, error) {
	service.identity = identity
	service.statusID = complaintID
	service.called = true
	return service.statusOptions, service.err
}

type stubTokenVerifier struct {
	identity authentication.VerifiedIdentity
}

func (verifier stubTokenVerifier) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return verifier.identity, nil
}

func TestControllerListsOpenCallCenterComplaints(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	service := &stubOpenCallCenterLister{results: []complaints.ComplaintSummary{{
		ComplaintID:      "TKT-001",
		ShipmentNumber:   "RESI-001",
		StatusCode:       "102",
		StatusDate:       time.Date(2026, time.September, 30, 11, 0, 0, 0, time.UTC),
		StatusOfficeCode: "40006CC",
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open?limit=10&offset=20&office_code=ATTACKER", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request complaint list: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !service.called || service.identity != identity {
		t.Fatal("controller did not pass the verified identity to the service")
	}
	if service.query.OfficeCode != "" || service.query.Limit != 10 || service.query.Offset != 20 {
		t.Fatalf("controller passed unexpected query: %#v", service.query)
	}
	var responseBody listResponse
	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(responseBody.Data) != 1 || responseBody.Data[0].ID != "TKT-001" || responseBody.Pagination.Limit != 10 || responseBody.Pagination.Offset != 20 {
		t.Fatalf("unexpected response: %#v", responseBody)
	}
}

func TestControllerListsReceivedAndSentDashboardRows(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	for _, path := range []string{
		"/api/v1/complaints/dashboard/received-open?from=2026-09-01&to=2026-09-30&limit=10&offset=20",
		"/api/v1/complaints/dashboard/sent-open?from=2026-09-01&to=2026-09-30&limit=10&offset=20",
	} {
		service := &stubOpenCallCenterLister{results: []complaints.ComplaintSummary{{ComplaintID: "TKT-001", StatusCode: "102"}}}
		app := newTestApp(identity, service)
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Authorization", "Bearer valid-token")
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("request %s: %v", path, err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if response.StatusCode != http.StatusOK || !service.called || service.identity != identity || service.query.Limit != 10 || service.query.Offset != 20 || service.query.From == nil || service.query.To == nil {
			t.Fatalf("%s response or service query unexpected: status=%d service=%#v", path, response.StatusCode, service)
		}
		if !strings.Contains(string(body), `"id":"TKT-001"`) || !strings.Contains(string(body), `"limit":10`) {
			t.Fatalf("%s response missing complaint or pagination: %s", path, body)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s Cache-Control = %q", path, response.Header.Get("Cache-Control"))
		}
	}
}

func TestControllerListsReceivedOpenDetailReport(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	createdAt := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	statusAt := time.Date(2026, time.September, 14, 9, 30, 0, 0, time.UTC)
	service := &stubOpenCallCenterLister{receivedReportRows: []complaints.ReceivedOpenReportRow{{
		ComplaintID: "TKT-001", TargetOfficeUpdateCode: "40005CC", OriginOfficeUpdateCode: "40006CC",
		ShipmentNumber: "RESI-001", ShipmentTypeCode: "REG", ComplaintCreatedAt: createdAt,
		ShipmentDestinationOfficeCode: "40007CC", LatestStatusCode: "102", LatestStatusDescription: "RESPON",
		LatestStatusOfficeCode: "40006CC", LatestStatusAt: statusAt, DurationSeconds: 176400,
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-open?from=2026-09-01&to=2026-09-30&product_code=REG&limit=10&offset=20", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.receivedReportCalled || service.identity != identity || service.receivedReportQuery.ProductCode != "REG" || service.receivedReportQuery.Limit != 10 || service.receivedReportQuery.Offset != 20 {
		t.Fatalf("report response/query unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(string(body), `"latest_status_code":"102"`) || !strings.Contains(string(body), `"duration_seconds":176400`) || strings.Contains(string(body), "customer_phone") {
		t.Fatalf("report response headers or projection unexpected: headers=%v body=%s", response.Header, body)
	}
}

func TestControllerRequiresReceivedOpenReportDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-open", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.receivedReportCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.receivedReportCalled)
	}
}

func TestControllerListsSentOpenDetailReport(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	createdAt := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	statusAt := time.Date(2026, time.September, 14, 9, 30, 0, 0, time.UTC)
	service := &stubOpenCallCenterLister{sentReportRows: []complaints.SentOpenReportRow{{
		ComplaintID: "TKT-001", OriginOfficeUpdateCode: "40005CC",
		ShipmentNumber: "RESI-001", ShipmentTypeCode: "REG", ComplaintCreatedAt: createdAt,
		ShipmentDestinationOfficeCode: "40007CC", LatestStatusCode: "102", LatestStatusDescription: "RESPON",
		LatestStatusOfficeCode: "40006CC", LatestStatusAt: statusAt, DurationSeconds: 176400,
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-open?from=2026-09-01&to=2026-09-30&product_code=REG&limit=10&offset=20", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.sentReportCalled || service.identity != identity || service.sentReportQuery.ProductCode != "REG" || service.sentReportQuery.Limit != 10 || service.sentReportQuery.Offset != 20 {
		t.Fatalf("report response/query unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(string(body), `"latest_status_code":"102"`) || !strings.Contains(string(body), `"origin_office_update_code":"40005CC"`) || !strings.Contains(string(body), `"duration_seconds":176400`) || strings.Contains(string(body), "customer_phone") {
		t.Fatalf("report response headers or projection unexpected: headers=%v body=%s", response.Header, body)
	}
}

func TestControllerRequiresSentOpenReportDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-open", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.sentReportCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.sentReportCalled)
	}
}

func TestControllerCountsReceivedOpenDurationBuckets(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	service := &stubOpenCallCenterLister{durationBuckets: complaints.ReceivedOpenDurationBuckets{
		DayZero: 1, DayOne: 2, DayTwo: 3, DayThree: 4, DayFourPlus: 5,
	}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-open/duration-buckets?from=2026-09-01&to=2026-09-30&product_code=REG", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.durationCalled || service.identity != identity || service.durationQuery.ProductCode != "REG" || service.durationQuery.From == nil || service.durationQuery.To == nil {
		t.Fatalf("duration request/response unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if !strings.Contains(string(body), `"day_zero_percent":6.67`) || !strings.Contains(string(body), `"day_four_plus_percent":33.33`) || !strings.Contains(string(body), `"total":15`) || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("duration response = %s, Cache-Control=%q", body, response.Header.Get("Cache-Control"))
	}
}

func TestReportPercentageReturnsZeroForEmptyReport(t *testing.T) {
	if got := reportPercentage(0, 0); got != 0 {
		t.Fatalf("reportPercentage(0, 0) = %v, want 0", got)
	}
}

func TestControllerRequiresReceivedOpenDurationDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-open/duration-buckets", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.durationCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.durationCalled)
	}
}

func TestControllerCountsSentOpenDurationBuckets(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	service := &stubOpenCallCenterLister{sentDurationBuckets: complaints.SentOpenDurationBuckets{
		DayZero: 1, DayOne: 2, DayTwo: 3, DayThree: 4, DayFourPlus: 5,
	}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-open/duration-buckets?from=2026-09-01&to=2026-09-30&product_code=REG", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.sentDurationCalled || service.identity != identity || service.sentDurationQuery.ProductCode != "REG" || service.sentDurationQuery.From == nil || service.sentDurationQuery.To == nil {
		t.Fatalf("duration request/response unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if !strings.Contains(string(body), `"day_zero_percent":6.67`) || !strings.Contains(string(body), `"day_four_plus_percent":33.33`) || !strings.Contains(string(body), `"total":15`) || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("duration response = %s, Cache-Control=%q", body, response.Header.Get("Cache-Control"))
	}
}

func TestControllerRequiresSentOpenDurationDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-open/duration-buckets", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.sentDurationCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.sentDurationCalled)
	}
}

func TestControllerListsSentSummaryWithRegionalAndProductFilters(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	service := &stubOpenCallCenterLister{sentSummaryRows: []complaints.SentSummaryRow{{
		OfficeCode: "40005CC", OfficeName: "Divre A", Total: 10, Completed: 6, Open: 4, Responded: 7, Unresponded: 3,
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-summary?from=2026-09-01&to=2026-09-30&product_code=REG&regional=4", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.sentSummaryCalled || service.sentSummaryQuery.RegionalCode != "4" || service.sentSummaryQuery.ProductCode != "REG" {
		t.Fatalf("summary request/response unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if !strings.Contains(string(body), `"office_code":"40005CC"`) || !strings.Contains(string(body), `"completed_percent":60`) || !strings.Contains(string(body), `"open_percent":40`) || !strings.Contains(string(body), `"responded_percent":70`) || !strings.Contains(string(body), `"unresponded_percent":30`) || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("summary response = %s, Cache-Control=%q", body, response.Header.Get("Cache-Control"))
	}
}

func TestControllerRequiresSentSummaryDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-summary", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.sentSummaryCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.sentSummaryCalled)
	}
}

func TestControllerListsReceivedSummaryWithRegionProductAndOnlyTarget(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	service := &stubOpenCallCenterLister{receivedSummaryRows: []complaints.ReceivedSummaryRow{{
		OfficeCode: "40006CC", OfficeName: "Divre B", Total: 10, Completed: 6, Open: 4, Responded: 7, Unresponded: 3,
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-summary?from=2026-09-01&to=2026-09-30&product_code=REG&regional=4&hanya_tujuan=on", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.receivedSummaryCalled || service.receivedSummaryQuery.RegionalCode != "4" || service.receivedSummaryQuery.ProductCode != "REG" || !service.receivedSummaryQuery.OnlyTarget {
		t.Fatalf("summary request/response unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if !strings.Contains(string(body), `"office_code":"40006CC"`) || !strings.Contains(string(body), `"completed_percent":60`) || !strings.Contains(string(body), `"open_percent":40`) || !strings.Contains(string(body), `"responded_percent":70`) || !strings.Contains(string(body), `"unresponded_percent":30`) || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("summary response = %s, Cache-Control=%q", body, response.Header.Get("Cache-Control"))
	}
}

func TestControllerRequiresReceivedSummaryDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-summary", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.receivedSummaryCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.receivedSummaryCalled)
	}
}

func TestControllerListsBreakdownWithDirectionKindAndOnlyTarget(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}
	service := &stubOpenCallCenterLister{breakdownRows: []complaints.BreakdownRow{
		{Code: "REG", Label: "Reguler", Count: 7},
		{Code: "EXP", Label: "Ekspres", Count: 3},
	}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/received-per-shipment?from=2026-09-01&to=2026-09-30&regional=4&hanya_tujuan=on", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.breakdownCalled || service.breakdownQuery.Direction != complaints.BreakdownDirectionReceived || service.breakdownQuery.Kind != complaints.BreakdownKindShipment || service.breakdownQuery.RegionalCode != "4" || !service.breakdownQuery.OnlyTarget {
		t.Fatalf("breakdown request/response unexpected: status=%d service=%#v", response.StatusCode, service)
	}
	if !strings.Contains(string(body), `"code":"REG"`) || !strings.Contains(string(body), `"percent":70`) || !strings.Contains(string(body), `"total":10`) || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("breakdown response = %s, Cache-Control=%q", body, response.Header.Get("Cache-Control"))
	}
}

func TestControllerMapsBreakdownKindForSentProblemRoute(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "regional.test", UserLevelID: "10"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-per-problem?from=2026-09-01&to=2026-09-30", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !service.breakdownCalled || service.breakdownQuery.Direction != complaints.BreakdownDirectionSent || service.breakdownQuery.Kind != complaints.BreakdownKindProblem || service.breakdownQuery.ProductCode != "" {
		t.Fatalf("sent problem breakdown service = %#v, status=%d", service, response.StatusCode)
	}
}

func TestControllerRequiresBreakdownDateBounds(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/reports/sent-per-shipment", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.breakdownCalled {
		t.Fatalf("status = %d, service called = %t, want 400 without service call", response.StatusCode, service.breakdownCalled)
	}
}

func TestControllerExportsBoundedOpenComplaintsAsSafeCSV(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	complaintDate := time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC)
	service := &stubOpenCallCenterLister{results: []complaints.ComplaintSummary{{
		ComplaintID:           "TKT-001",
		OriginOfficeCode:      "40005CC",
		ShipmentNumber:        "RESI-001",
		ComplaintDate:         &complaintDate,
		Summary:               `=HYPERLINK("https://invalid.test")`,
		StatusCode:            "102",
		StatusDate:            time.Date(2026, time.September, 30, 11, 0, 0, 0, time.UTC),
		StatusOfficeCode:      "40006CC",
		AllTargetOfficeCodes:  "40006CC,40005CC",
		DestinationOfficeCode: "40006CC",
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open/export?limit=10&offset=20", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !service.exportCalled || service.identity != identity || service.query.Limit != 10 || service.query.Offset != 20 {
		t.Fatalf("export response/status or service request = %d/%#v", response.StatusCode, service)
	}
	if response.Header.Get("Content-Type") != "text/csv; charset=utf-8" || response.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(response.Header.Get("Content-Disposition"), "cch-open-complaints.csv") {
		t.Fatalf("unexpected export headers: %#v", response.Header)
	}
	rows, err := csv.NewReader(response.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(rows[0]) != 11 || rows[1][0] != "TKT-001" || rows[1][7] != `'=HYPERLINK("https://invalid.test")` {
		t.Fatalf("unexpected CSV rows: %#v", rows)
	}
}

func TestSpreadsheetSafeCellPrefixesFormulaLikeValues(t *testing.T) {
	for _, value := range []string{"=1+1", " +SUM(A1:A2)", "-1+1", "@SUM(A1:A2)", "\t=1+1"} {
		if got := spreadsheetSafeCell(value); !strings.HasPrefix(got, "'") {
			t.Errorf("spreadsheetSafeCell(%q) = %q, want a text prefix", value, got)
		}
	}
	if got := spreadsheetSafeCell("ordinary text"); got != "ordinary text" {
		t.Errorf("spreadsheetSafeCell ordinary value = %q", got)
	}
}

func TestControllerRegistersReceivedAndSentOpenRequestCounts(t *testing.T) {
	for _, path := range []string{
		"/api/v1/complaints/call-center/received-open/count?from=2026-09-01&to=2026-09-30",
		"/api/v1/complaints/call-center/sent-open/count?from=2026-09-01&to=2026-09-30",
	} {
		service := &stubOpenCallCenterLister{results: []complaints.ComplaintSummary{{ComplaintID: "TKT-001"}}}
		app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}, service)
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Authorization", "Bearer valid-token")
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("request %s: %v", path, err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if response.StatusCode != http.StatusOK || string(body) != `{"count":1}` {
			t.Fatalf("%s response = %d %s, want 200 {\"count\":1}", path, response.StatusCode, body)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s Cache-Control = %q, want private, no-store", path, response.Header.Get("Cache-Control"))
		}
	}
}

func TestControllerRegistersDashboardCountVariants(t *testing.T) {
	for _, path := range []string{
		"/api/v1/complaints/dashboard/open/count",
		"/api/v1/complaints/dashboard/received-open/count",
		"/api/v1/complaints/dashboard/sent-open/count",
		"/api/v1/complaints/dashboard/completed/count",
	} {
		service := &stubOpenCallCenterLister{results: []complaints.ComplaintSummary{{ComplaintID: "TKT-001"}}}
		app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC", UserLevelID: "10"}, service)
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Authorization", "Bearer valid-token")
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("request %s: %v", path, err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if response.StatusCode != http.StatusOK || string(body) != `{"count":1}` {
			t.Fatalf("%s response = %d %s, want 200 {\"count\":1}", path, response.StatusCode, body)
		}
	}
}

func TestControllerRequiresVerifiedIdentity(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{}, service)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open", nil))
	if err != nil {
		t.Fatalf("request complaint list without token: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called {
		t.Fatalf("status = %d, service called = %v; want 401 without service call", response.StatusCode, service.called)
	}
}

func TestControllerRejectsInvalidPagination(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open?limit=101", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request invalid pagination: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.called {
		t.Fatalf("status = %d, service called = %v; want 400 without service call", response.StatusCode, service.called)
	}
}

func TestControllerListsCallCenterTicketHistoryWithTicketIDAndPagination(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	updatedAt := time.Date(2026, time.September, 29, 8, 30, 0, 0, time.UTC)
	service := &stubOpenCallCenterLister{historyEntries: []complaints.ComplaintStatusHistoryEntry{{
		ComplaintID:       "TKT-001",
		HistoryID:         "HIST-002",
		StatusCode:        "102",
		StatusDescription: "RESPON",
		OriginOfficeCode:  "40006CC",
		Description:       "Sedang ditangani",
		UpdatedAt:         updatedAt,
		UpdatedBy:         "cs.agent",
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/history?limit=10&offset=20&office_code=ATTACKER", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request status history: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !service.historyCalled || service.identity != identity {
		t.Fatal("controller did not pass the verified identity to the history service")
	}
	if service.historyQuery.ComplaintID != "TKT-001" || service.historyQuery.OfficeCode != "" || service.historyQuery.Limit != 10 || service.historyQuery.Offset != 20 {
		t.Fatalf("controller passed an unexpected history query: %#v", service.historyQuery)
	}
	var responseBody historyResponse
	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	if len(responseBody.Data) != 1 || responseBody.Data[0].HistoryID != "HIST-002" || responseBody.Data[0].StatusDescription != "RESPON" || responseBody.Pagination.Limit != 10 {
		t.Fatalf("unexpected history response: %#v", responseBody)
	}
}

func TestControllerRejectsInvalidHistoryTicketID(t *testing.T) {
	service := &stubOpenCallCenterLister{}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/12345678901234567890123456/history", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request invalid history ID: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.historyCalled {
		t.Fatalf("status = %d, history service called = %v; want 400 without service call", response.StatusCode, service.historyCalled)
	}
}

func TestControllerGetsCallCenterTicketDetailWithVerifiedIdentity(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	service := &stubOpenCallCenterLister{detail: complaints.ComplaintDetail{
		ComplaintID:      "TKT-001",
		CustomerName:     "Nama Pelanggan",
		CustomerPhone:    "0800000000",
		Description:      "Uraian pengaduan",
		StatusCode:       "102",
		StatusDate:       time.Date(2026, time.September, 30, 11, 0, 0, 0, time.UTC),
		StatusOfficeCode: "40006CC",
	}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001?office_code=ATTACKER", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request complaint detail: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !service.detailCalled || service.detailID != "TKT-001" || service.identity != identity {
		t.Fatal("controller did not pass ticket ID and verified identity to detail service")
	}
	var body detailResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if body.Data.ID != "TKT-001" || body.Data.CustomerName != "Nama Pelanggan" || body.Data.CustomerPhone != "0800000000" || body.Data.StatusOfficeCode != "40006CC" {
		t.Fatalf("unexpected detail response: %#v", body.Data)
	}
}

func TestControllerMapsInvisibleComplaintToNotFound(t *testing.T) {
	service := &stubOpenCallCenterLister{err: complaints.ErrComplaintNotFoundOrNotVisible}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request invisible complaint: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func TestControllerListsCallCenterStatusOptionsWithoutCaching(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	service := &stubOpenCallCenterLister{statusOptions: []complaints.ComplaintStatusOption{
		{Code: "103", Description: "KONFIRMASI"},
		{Code: "109", Description: "SELESAI"},
	}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/status-options", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" || service.statusID != "TKT-001" || service.identity != identity {
		t.Fatalf("status=%d cache=%q service=%#v", response.StatusCode, response.Header.Get("Cache-Control"), service)
	}
	var body statusOptionsResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 2 || body.Data[0].Code != "103" || body.Data[1].Code != "109" {
		t.Fatalf("unexpected status options: %#v", body.Data)
	}
}

func TestControllerHidesUnavailableCallCenterStatusOptions(t *testing.T) {
	service := &stubOpenCallCenterLister{err: complaints.ErrStatusOptionsNotAvailable}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-OUTSIDE/status-options", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for unavailable status options", response.StatusCode)
	}
}

func TestControllerMapsPolicyErrorsAndHidesBackendDetails(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantHidden string
	}{
		{name: "unsupported role", serviceErr: complaints.ErrCallCenterRoleRequired, wantStatus: http.StatusForbidden},
		{name: "database failure", serviceErr: errors.New("database password detail"), wantStatus: http.StatusServiceUnavailable, wantHidden: "database password detail"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &stubOpenCallCenterLister{err: test.serviceErr}
			app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open", nil)
			request.Header.Set("X-Authorization", "valid-token")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("request complaint list: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantHidden != "" {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatalf("read error response: %v", err)
				}
				if strings.Contains(string(body), test.wantHidden) {
					t.Fatal("internal error detail was exposed")
				}
			}
		})
	}
}

func newTestApp(identity authentication.VerifiedIdentity, service *stubOpenCallCenterLister) *fiber.App {
	app := httpapi.NewServer(nil)
	app.Use(httpapi.JWTAuthentication(stubTokenVerifier{identity: identity}))
	New(service).Register(app.Group("/api/v1"))
	return app
}
