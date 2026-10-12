package complaints

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	DefaultComplaintPageSize = 25
	MaxComplaintPageSize     = 100
)

type ComplaintOfficeMatch uint8

const (
	MatchAllRoutingFields ComplaintOfficeMatch = iota
	MatchOriginOrAllTargets
	MatchAllTargetsOnly
	MatchOriginOnly
)

var ErrInvalidComplaintListQuery = errors.New("invalid complaint list query")

type ComplaintListQuery struct {
	OfficeCode    string
	OfficeMatch   ComplaintOfficeMatch
	OpenOnly      bool
	CompletedOnly bool
	From          *time.Time
	To            *time.Time
	Limit         int
	Offset        int
}

func (query ComplaintListQuery) Normalize() (ComplaintListQuery, error) {
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return ComplaintListQuery{}, ErrInvalidComplaintListQuery
	}
	if query.OfficeMatch > MatchOriginOnly {
		return ComplaintListQuery{}, ErrInvalidComplaintListQuery
	}
	if query.OpenOnly && query.CompletedOnly {
		return ComplaintListQuery{}, ErrInvalidComplaintListQuery
	}
	dateRange, err := (CompletedCountQuery{From: query.From, To: query.To}).Normalize()
	if err != nil {
		return ComplaintListQuery{}, ErrInvalidComplaintListQuery
	}
	query.From = dateRange.From
	query.To = dateRange.To
	return query.NormalizePagination()
}

func (query ComplaintListQuery) NormalizePagination() (ComplaintListQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return ComplaintListQuery{}, ErrInvalidComplaintListQuery
	}
	return query, nil
}

type ComplaintSummary struct {
	ComplaintID           string
	OriginOfficeCode      string
	DestinationOfficeCode string
	CopyOfficeCodes       string
	AllTargetOfficeCodes  string
	ShipmentNumber        string
	ComplaintDate         *time.Time
	Summary               string
	StatusCode            string
	StatusDate            time.Time
	StatusOfficeCode      string
}

type ReceivedOpenReportQuery struct {
	From        *time.Time
	To          *time.Time
	ProductCode string
	Limit       int
	Offset      int
}

func (query ReceivedOpenReportQuery) Normalize() (ReceivedOpenReportQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") {
		return ReceivedOpenReportQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return ReceivedOpenReportQuery{}, ErrInvalidComplaintListQuery
	}
	return query, nil
}

type ReceivedOpenReportRow struct {
	ComplaintID                   string
	TargetOfficeUpdateCode        string
	OriginOfficeUpdateCode        string
	ShipmentNumber                string
	ShipmentTypeCode              string
	ComplaintCreatedAt            time.Time
	ShipmentDestinationOfficeCode string
	LatestStatusCode              string
	LatestStatusDescription       string
	LatestStatusOfficeCode        string
	LatestStatusAt                time.Time
	DurationSeconds               int64
}

type SentOpenReportQuery struct {
	From        *time.Time
	To          *time.Time
	ProductCode string
	Limit       int
	Offset      int
}

func (query SentOpenReportQuery) Normalize() (SentOpenReportQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") {
		return SentOpenReportQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return SentOpenReportQuery{}, ErrInvalidComplaintListQuery
	}
	return query, nil
}

type SentOpenReportRow struct {
	ComplaintID                   string
	OriginOfficeUpdateCode        string
	ShipmentNumber                string
	ShipmentTypeCode              string
	ComplaintCreatedAt            time.Time
	ShipmentDestinationOfficeCode string
	LatestStatusCode              string
	LatestStatusDescription       string
	LatestStatusOfficeCode        string
	LatestStatusAt                time.Time
	DurationSeconds               int64
}

type SentSummaryQuery struct {
	From         *time.Time
	To           *time.Time
	ProductCode  string
	RegionalCode string
	Limit        int
	Offset       int
}

func (query SentSummaryQuery) Normalize() (SentSummaryQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	query.RegionalCode = strings.TrimSpace(query.RegionalCode)
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") ||
		len(query.RegionalCode) > 20 || strings.Contains(query.RegionalCode, ",") {
		return SentSummaryQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	if query.RegionalCode == "0" {
		query.RegionalCode = ""
	}
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return SentSummaryQuery{}, ErrInvalidComplaintListQuery
	}
	return query, nil
}

type SentSummaryRow struct {
	OfficeCode  string
	OfficeName  string
	Total       int64
	Completed   int64
	Open        int64
	Responded   int64
	Unresponded int64
}

type ReceivedSummaryQuery struct {
	From         *time.Time
	To           *time.Time
	ProductCode  string
	RegionalCode string
	OnlyTarget   bool
	Limit        int
	Offset       int
}

func (query ReceivedSummaryQuery) Normalize() (ReceivedSummaryQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	query.RegionalCode = strings.TrimSpace(query.RegionalCode)
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") ||
		len(query.RegionalCode) > 20 || strings.Contains(query.RegionalCode, ",") {
		return ReceivedSummaryQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	if query.RegionalCode == "0" {
		query.RegionalCode = ""
	}
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return ReceivedSummaryQuery{}, ErrInvalidComplaintListQuery
	}
	return query, nil
}

type ReceivedSummaryRow struct {
	OfficeCode  string
	OfficeName  string
	Total       int64
	Completed   int64
	Open        int64
	Responded   int64
	Unresponded int64
}

type BreakdownDirection string

const (
	BreakdownDirectionSent     BreakdownDirection = "sent"
	BreakdownDirectionReceived BreakdownDirection = "received"
)

type BreakdownKind string

const (
	BreakdownKindShipment BreakdownKind = "shipment"
	BreakdownKindProblem  BreakdownKind = "problem"
)

type BreakdownQuery struct {
	Direction    BreakdownDirection
	Kind         BreakdownKind
	From         *time.Time
	To           *time.Time
	ProductCode  string
	RegionalCode string
	OnlyTarget   bool
	Limit        int
	Offset       int
}

func (query BreakdownQuery) Normalize() (BreakdownQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	query.RegionalCode = strings.TrimSpace(query.RegionalCode)
	if query.Direction != BreakdownDirectionSent && query.Direction != BreakdownDirectionReceived {
		return BreakdownQuery{}, ErrInvalidComplaintListQuery
	}
	if query.Kind != BreakdownKindShipment && query.Kind != BreakdownKindProblem {
		return BreakdownQuery{}, ErrInvalidComplaintListQuery
	}
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") ||
		len(query.RegionalCode) > 20 || strings.Contains(query.RegionalCode, ",") {
		return BreakdownQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	if query.RegionalCode == "0" {
		query.RegionalCode = ""
	}
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return BreakdownQuery{}, ErrInvalidComplaintListQuery
	}
	return query, nil
}

type BreakdownRow struct {
	Code  string
	Label string
	Count int64
}

type ReceivedOpenDurationQuery struct {
	From        *time.Time
	To          *time.Time
	ProductCode string
}

func (query ReceivedOpenDurationQuery) Normalize() (ReceivedOpenDurationQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") {
		return ReceivedOpenDurationQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	return query, nil
}

type ReceivedOpenDurationBuckets struct {
	DayZero     int64
	DayOne      int64
	DayTwo      int64
	DayThree    int64
	DayFourPlus int64
}

func (buckets ReceivedOpenDurationBuckets) Total() int64 {
	return buckets.DayZero + buckets.DayOne + buckets.DayTwo + buckets.DayThree + buckets.DayFourPlus
}

type SentOpenDurationQuery struct {
	From        *time.Time
	To          *time.Time
	ProductCode string
}

func (query SentOpenDurationQuery) Normalize() (SentOpenDurationQuery, error) {
	query.ProductCode = strings.TrimSpace(query.ProductCode)
	if query.From == nil || query.To == nil || query.To.Before(*query.From) ||
		len(query.ProductCode) > 20 || strings.Contains(query.ProductCode, ",") {
		return SentOpenDurationQuery{}, ErrInvalidComplaintListQuery
	}
	if query.ProductCode == "999" {
		query.ProductCode = ""
	}
	return query, nil
}

type SentOpenDurationBuckets = ReceivedOpenDurationBuckets

type CompletedCountQuery struct {
	From *time.Time
	To   *time.Time
}

type OpenCountQuery = CompletedCountQuery

func (query CompletedCountQuery) Normalize() (CompletedCountQuery, error) {
	if query.From == nil && query.To == nil {
		return query, nil
	}
	if query.From == nil || query.To == nil || query.To.Before(*query.From) {
		return CompletedCountQuery{}, errors.New("invalid completed count date range")
	}
	return query, nil
}

type ComplaintRepository interface {
	ListInvolvingOffice(context.Context, ComplaintListQuery) ([]ComplaintSummary, error)
	ListReceivedOpenReport(context.Context, ReceivedOpenReportQuery, string) ([]ReceivedOpenReportRow, error)
	ListSentOpenReport(context.Context, SentOpenReportQuery, string) ([]SentOpenReportRow, error)
	ListSentSummary(context.Context, SentSummaryQuery, string) ([]SentSummaryRow, error)
	ListReceivedSummary(context.Context, ReceivedSummaryQuery, string) ([]ReceivedSummaryRow, error)
	ListBreakdown(context.Context, BreakdownQuery, string) ([]BreakdownRow, error)
	CountReceivedOpenDurationBuckets(context.Context, ReceivedOpenDurationQuery, string) (ReceivedOpenDurationBuckets, error)
	CountSentOpenDurationBuckets(context.Context, SentOpenDurationQuery, string) (SentOpenDurationBuckets, error)
	ListOpenTicketHistory(context.Context, ComplaintHistoryQuery) ([]ComplaintStatusHistoryEntry, error)
	ListCompletedTicketHistory(context.Context, ComplaintHistoryQuery) ([]ComplaintStatusHistoryEntry, error)
	CountCompletedForCallCenter(context.Context, CompletedCountQuery, string) (int64, error)
	CountOpenForCallCenter(context.Context, OpenCountQuery, string) (int64, error)
	CountReceivedOpenForCallCenter(context.Context, OpenCountQuery, string) (int64, error)
	CountSentOpenForCallCenter(context.Context, OpenCountQuery, string) (int64, error)
	GetOpenTicketForOffice(context.Context, ComplaintDetailQuery) (ComplaintDetail, error)
	GetCompletedTicketForOffice(context.Context, ComplaintDetailQuery) (ComplaintDetail, error)
	ListCallCenterStatusOptions(context.Context, string, string) ([]ComplaintStatusOption, error)
}

type ComplaintDetailQuery struct {
	ComplaintID string
	OfficeCode  string
}

func (query ComplaintDetailQuery) Normalize() (ComplaintDetailQuery, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return ComplaintDetailQuery{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return ComplaintDetailQuery{}, ErrInvalidComplaintDetailQuery
	}
	return query, nil
}

func (query ComplaintDetailQuery) NormalizeRequest() (ComplaintDetailQuery, error) {
	query.ComplaintID = strings.TrimSpace(query.ComplaintID)
	if query.ComplaintID == "" || len(query.ComplaintID) > 25 || strings.Contains(query.ComplaintID, ",") {
		return ComplaintDetailQuery{}, ErrInvalidComplaintDetailQuery
	}
	return query, nil
}

type ComplaintDetail struct {
	ComplaintID           string
	CustomerName          string
	CustomerEmail         string
	CustomerPhone         string
	CustomerAddress       string
	OriginOfficeCode      string
	DestinationOfficeCode string
	CopyOfficeCodes       string
	AllTargetOfficeCodes  string
	ComplaintTypeCode     int64
	ComplaintSourceCode   int64
	ShipmentTypeCode      string
	ShipmentNumber        string
	ComplaintDate         *time.Time
	ShipmentOriginOffice  string
	ShipmentTargetOffice  string
	Summary               string
	Description           string
	StatusCode            string
	StatusDescription     string
	StatusDate            time.Time
	StatusOfficeCode      string
	RequestStatus         string
}

var (
	ErrInvalidComplaintDetailQuery   = errors.New("invalid complaint detail query")
	ErrComplaintNotFoundOrNotVisible = errors.New("complaint not found or not visible")
)

type ComplaintHistoryQuery struct {
	ComplaintID string
	OfficeCode  string
	Limit       int
	Offset      int
}

func (query ComplaintHistoryQuery) Normalize() (ComplaintHistoryQuery, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return ComplaintHistoryQuery{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return ComplaintHistoryQuery{}, ErrInvalidComplaintHistoryQuery
	}
	return query, nil
}

func (query ComplaintHistoryQuery) NormalizeRequest() (ComplaintHistoryQuery, error) {
	query.ComplaintID = strings.TrimSpace(query.ComplaintID)
	if query.ComplaintID == "" || len(query.ComplaintID) > 25 || strings.Contains(query.ComplaintID, ",") {
		return ComplaintHistoryQuery{}, ErrInvalidComplaintHistoryQuery
	}
	if query.Limit == 0 {
		query.Limit = DefaultComplaintPageSize
	}
	if query.Limit < 1 || query.Limit > MaxComplaintPageSize || query.Offset < 0 {
		return ComplaintHistoryQuery{}, ErrInvalidComplaintHistoryQuery
	}
	return query, nil
}

type ComplaintStatusHistoryEntry struct {
	ComplaintID       string
	HistoryID         string
	StatusCode        string
	StatusDescription string
	OriginOfficeCode  string
	Description       string
	UpdatedAt         time.Time
	UpdatedBy         string
	DurationSeconds   *int64
}

var ErrInvalidComplaintHistoryQuery = errors.New("invalid complaint history query")

type ComplaintStatusOption struct {
	Code        string
	Description string
}

var ErrStatusOptionsNotAvailable = errors.New("status options are not available for this complaint")
