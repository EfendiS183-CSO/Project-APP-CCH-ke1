package complaints

import (
	"errors"
	"testing"
)

func TestComplaintListQueryNormalizesOfficeAndDefaultPageSize(t *testing.T) {
	query, err := (ComplaintListQuery{OfficeCode: " 40005CC "}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if query.OfficeCode != "40005CC" || query.Limit != DefaultComplaintPageSize || query.Offset != 0 {
		t.Fatalf("normalized query = %#v", query)
	}
}

func TestComplaintListQueryRejectsInvalidOfficeAndPagination(t *testing.T) {
	tests := []struct {
		name  string
		query ComplaintListQuery
	}{
		{name: "empty office"},
		{name: "office too long", query: ComplaintListQuery{OfficeCode: "12345678"}},
		{name: "office contains list delimiter", query: ComplaintListQuery{OfficeCode: "40005CC,40006CC"}},
		{name: "negative limit", query: ComplaintListQuery{OfficeCode: "40005CC", Limit: -1}},
		{name: "limit too large", query: ComplaintListQuery{OfficeCode: "40005CC", Limit: MaxComplaintPageSize + 1}},
		{name: "negative offset", query: ComplaintListQuery{OfficeCode: "40005CC", Offset: -1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.query.Normalize(); !errors.Is(err, ErrInvalidComplaintListQuery) {
				t.Fatalf("Normalize() error = %v, want invalid query", err)
			}
		})
	}
}

func TestComplaintHistoryQueryNormalizesCallerFields(t *testing.T) {
	query, err := (ComplaintHistoryQuery{ComplaintID: " TKT-001 ", Limit: 0}).NormalizeRequest()
	if err != nil {
		t.Fatal(err)
	}
	if query.ComplaintID != "TKT-001" || query.Limit != DefaultComplaintPageSize || query.Offset != 0 {
		t.Fatalf("normalized history request = %#v", query)
	}
}

func TestComplaintHistoryQueryRejectsInvalidRequestAndOffice(t *testing.T) {
	for _, test := range []struct {
		name  string
		query ComplaintHistoryQuery
	}{
		{name: "empty ticket ID", query: ComplaintHistoryQuery{OfficeCode: "40005CC"}},
		{name: "ticket ID too long", query: ComplaintHistoryQuery{ComplaintID: "12345678901234567890123456"}},
		{name: "ticket ID delimiter", query: ComplaintHistoryQuery{ComplaintID: "TKT-1,TKT-2"}},
		{name: "invalid page size", query: ComplaintHistoryQuery{ComplaintID: "TKT-1", Limit: MaxComplaintPageSize + 1}},
		{name: "invalid office", query: ComplaintHistoryQuery{ComplaintID: "TKT-1", OfficeCode: ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var err error
			if test.name == "invalid office" {
				_, err = test.query.Normalize()
			} else {
				_, err = test.query.NormalizeRequest()
			}
			if !errors.Is(err, ErrInvalidComplaintHistoryQuery) {
				t.Fatalf("Normalize() error = %v, want invalid history query", err)
			}
		})
	}
}
