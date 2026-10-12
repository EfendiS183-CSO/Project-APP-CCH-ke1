package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/complaints"
)

func TestMySQLComplaintRepositoryListsOfficeAssociationsWithBoundedProjection(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	complaintDate := time.Date(2026, time.September, 30, 10, 15, 0, 0, time.UTC)
	statusDate := time.Date(2026, time.September, 30, 11, 0, 0, 0, time.UTC)
	query := complaints.ComplaintListQuery{OfficeCode: "40005CC", Limit: complaints.DefaultComplaintPageSize}
	statement, _ := buildComplaintListQuery(query)
	mock.ExpectQuery(regexp.QuoteMeta(statement)).
		WithArgs("40005CC", "40005CC", "40005CC", "40005CC", complaints.DefaultComplaintPageSize, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "Kantor_Asal", "Kantor_Tujuan", "Kantor_Tembusan", "Semua_Tujuan",
			"Nomor_Kiriman", "Tanggal_Pengaduan", "Ringkasan_Pengaduan", "Status_Akhir", "Tanggal_Status", "Kantor_Status",
		}).AddRow("TKT-001", "40005CC", "40006CC", "40007CC,40005CC", "40006CC,40005CC", "RESI-001", complaintDate, "Kiriman terlambat", "102", statusDate, "40006CC"))

	query.OfficeCode = " 40005CC "
	results, err := NewMySQLComplaintRepository(database).ListInvolvingOffice(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].ComplaintID != "TKT-001" || results[0].StatusCode != "102" || results[0].StatusOfficeCode != "40006CC" {
		t.Fatalf("unexpected complaint summary: %#v", results[0])
	}
	if results[0].ComplaintDate == nil || !results[0].ComplaintDate.Equal(complaintDate) || !results[0].StatusDate.Equal(statusDate) {
		t.Fatalf("unexpected complaint dates: %#v", results[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLComplaintRepositoryRejectsInvalidQueryBeforeDatabaseAccess(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	_, err = NewMySQLComplaintRepository(database).ListInvolvingOffice(context.Background(), complaints.ComplaintListQuery{OfficeCode: "40005CC,40006CC"})
	if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
		t.Fatalf("ListInvolvingOffice() error = %v, want invalid query", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLComplaintRepositoryListsReceivedOpenReportWithLatestStatusAndFilters(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	createdAt := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.September, 14, 9, 30, 0, 0, time.UTC)
	statement := receivedOpenReportSQL + "\n\tAND c.Jenis_Kiriman = ?\nORDER BY c.Tanggal_Tambah DESC, c.ID_Pengaduan DESC LIMIT ? OFFSET ?"
	mock.ExpectQuery(regexp.QuoteMeta(statement)).
		WithArgs("40005CC", from, to, "REG", 10, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "Kantor_Tujuan_Update", "Kantor_Asal_Update", "Nomor_Kiriman", "Jenis_Kiriman",
			"Tanggal_Tambah", "Kantor_Tujuan", "Status_Update", "Deskripsi_Status", "Kantor_Asal_Update_Latest",
			"Tanggal_Update", "DurationSeconds",
		}).AddRow("TKT-001", "40005CC", "40006CC", "RESI-001", "REG", createdAt, "40007CC", "102", "RESPON", "40006CC", updatedAt, int64(176400)))

	results, err := NewMySQLComplaintRepository(database).ListReceivedOpenReport(context.Background(), complaints.ReceivedOpenReportQuery{
		From: &from, To: &to, ProductCode: "REG", Limit: 10, Offset: 20,
	}, "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ComplaintID != "TKT-001" || results[0].LatestStatusCode != "102" || results[0].LatestStatusOfficeCode != "40006CC" || results[0].DurationSeconds != 176400 {
		t.Fatalf("unexpected received-open report rows: %#v", results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReceivedOpenReportRequiresDateBoundsAndRejectsInvalidOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLComplaintRepository(database)
	validFrom := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		query  complaints.ReceivedOpenReportQuery
		office string
	}{
		{name: "missing date range", query: complaints.ReceivedOpenReportQuery{Limit: 10}, office: "40005CC"},
		{name: "reversed date range", query: complaints.ReceivedOpenReportQuery{From: &validTo, To: &validFrom, Limit: 10}, office: "40005CC"},
		{name: "spoofable multi-office value", query: complaints.ReceivedOpenReportQuery{From: &validFrom, To: &validTo, Limit: 10}, office: "40005CC,40006CC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := repository.ListReceivedOpenReport(context.Background(), test.query, test.office)
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("ListReceivedOpenReport() error = %v, want invalid report query", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCountsReceivedOpenDurationBuckets(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name        string
		productCode string
		extraSQL    string
		args        []driver.Value
	}{
		{name: "single product", productCode: "REG", extraSQL: "\n\t\tAND c.Jenis_Kiriman = ?", args: []driver.Value{"REG"}},
		{name: "all products sentinel", productCode: "999", args: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			statement := receivedOpenDurationBucketsSQL + test.extraSQL + "\n) AS eligible"
			arguments := []driver.Value{"40005CC", "40005CC", from, to}
			arguments = append(arguments, test.args...)
			mock.ExpectQuery(regexp.QuoteMeta(statement)).WithArgs(arguments...).
				WillReturnRows(sqlmock.NewRows([]string{"day_zero", "day_one", "day_two", "day_three", "day_four_plus"}).AddRow(1, 2, 3, 4, 5))

			buckets, err := NewMySQLComplaintRepository(database).CountReceivedOpenDurationBuckets(context.Background(), complaints.ReceivedOpenDurationQuery{
				From: &from, To: &to, ProductCode: test.productCode,
			}, "40005CC")
			if err != nil {
				t.Fatal(err)
			}
			if buckets.DayZero != 1 || buckets.DayOne != 2 || buckets.DayTwo != 3 || buckets.DayThree != 4 || buckets.DayFourPlus != 5 || buckets.Total() != 15 {
				t.Fatalf("duration buckets = %#v, total=%d", buckets, buckets.Total())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestListReceivedSummaryGroupsByTargetOfficeWithFilters(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name         string
		regionalCode string
		productCode  string
		onlyTarget   bool
		innerExtra   string
		outerExtra   string
		args         []driver.Value
	}{
		{name: "all regions and products", regionalCode: "0", productCode: "999"},
		{name: "single product and region with only-target",
			regionalCode: "4", productCode: "REG", onlyTarget: true,
			innerExtra: "\n\t\tAND e.Jenis_Kiriman = ?\n\t\tAND e.Kantor_Tujuan = u.Kantor_Tujuan_Update",
			outerExtra: "\n\tAND ((k.Regional = ?) OR (k.Kode_Kantor = ?))",
			args:       []driver.Value{"REG", "4", "4"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			statement := receivedSummarySQL + test.innerExtra + "\n) AS t\nINNER JOIN cch_kantor AS k ON k.Kode_Kantor = t.OfficeCode" + test.outerExtra +
				"\nGROUP BY t.OfficeCode, k.Nama_Kantor\nORDER BY t.OfficeCode ASC\nLIMIT ? OFFSET ?"
			arguments := []driver.Value{from, to}
			arguments = append(arguments, test.args...)
			arguments = append(arguments, 10, 0)
			mock.ExpectQuery(regexp.QuoteMeta(statement)).WithArgs(arguments...).
				WillReturnRows(sqlmock.NewRows([]string{"OfficeCode", "OfficeName", "Total", "Completed", "Unresponded"}).
					AddRow("40006CC", "Divre B", 10, 6, 3))

			results, err := NewMySQLComplaintRepository(database).ListReceivedSummary(context.Background(), complaints.ReceivedSummaryQuery{
				From: &from, To: &to, ProductCode: test.productCode, RegionalCode: test.regionalCode, OnlyTarget: test.onlyTarget, Limit: 10, Offset: 0,
			}, "40005CC")
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].OfficeCode != "40006CC" || results[0].Total != 10 || results[0].Completed != 6 || results[0].Open != 4 || results[0].Responded != 7 || results[0].Unresponded != 3 {
				t.Fatalf("unexpected received summary rows: %#v", results)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReceivedSummaryRejectsInvalidQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLComplaintRepository(database)
	validFrom := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	for _, probe := range []struct {
		name   string
		query  complaints.ReceivedSummaryQuery
		office string
	}{
		{name: "missing date range", query: complaints.ReceivedSummaryQuery{Limit: 10}, office: "40005CC"},
		{name: "region list injection", query: complaints.ReceivedSummaryQuery{From: &validFrom, To: &validTo, RegionalCode: "4,5", Limit: 10}, office: "40005CC"},
		{name: "multi-office spoof", query: complaints.ReceivedSummaryQuery{From: &validFrom, To: &validTo, Limit: 10}, office: "40005CC,40006CC"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			_, err := repository.ListReceivedSummary(context.Background(), probe.query, probe.office)
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("ListReceivedSummary() error = %v, want invalid query", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReceivedOpenDurationBucketsRejectInvalidBoundsBeforeQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	validDate := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		query  complaints.ReceivedOpenDurationQuery
		office string
	}{
		{name: "missing end date", query: complaints.ReceivedOpenDurationQuery{From: &validDate}, office: "40005CC"},
		{name: "invalid office list", query: complaints.ReceivedOpenDurationQuery{From: &validDate, To: &validDate}, office: "40005CC,40006CC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewMySQLComplaintRepository(database).CountReceivedOpenDurationBuckets(context.Background(), test.query, test.office)
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("CountReceivedOpenDurationBuckets() error = %v, want invalid query", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCountSentOpenDurationBucketsScopesByOriginOfficeAndProduct(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name        string
		productCode string
		extraSQL    string
		args        []driver.Value
	}{
		{name: "all products", productCode: "999"},
		{name: "single product", productCode: "REG", extraSQL: "\n\t\tAND c.Jenis_Kiriman = ?", args: []driver.Value{"REG"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			statement := sentOpenDurationBucketsSQL + test.extraSQL + "\n) AS eligible"
			arguments := []driver.Value{"40005CC", from, to}
			arguments = append(arguments, test.args...)
			mock.ExpectQuery(regexp.QuoteMeta(statement)).WithArgs(arguments...).
				WillReturnRows(sqlmock.NewRows([]string{"day_zero", "day_one", "day_two", "day_three", "day_four_plus"}).AddRow(1, 2, 3, 4, 5))

			buckets, err := NewMySQLComplaintRepository(database).CountSentOpenDurationBuckets(context.Background(), complaints.SentOpenDurationQuery{
				From: &from, To: &to, ProductCode: test.productCode,
			}, "40005CC")
			if err != nil {
				t.Fatal(err)
			}
			if buckets.DayZero != 1 || buckets.DayOne != 2 || buckets.DayTwo != 3 || buckets.DayThree != 4 || buckets.DayFourPlus != 5 || buckets.Total() != 15 {
				t.Fatalf("duration buckets = %#v, total=%d", buckets, buckets.Total())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestListBreakdownBuildsDirectionAndKindQueries(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name      string
		statement string
		query     complaints.BreakdownQuery
		wantArgs  []driver.Value
	}{
		{
			name:      "sent shipment all",
			statement: sentBreakdownShipmentSQL + "\nGROUP BY Code, Label\nORDER BY Count DESC, Code ASC\nLIMIT ? OFFSET ?",
			query:     complaints.BreakdownQuery{Direction: complaints.BreakdownDirectionSent, Kind: complaints.BreakdownKindShipment, From: &from, To: &to, Limit: 10, Offset: 0},
			wantArgs:  []driver.Value{from, to, 10, 0},
		},
		{
			name:      "sent problem with product and region",
			statement: sentBreakdownProblemSQL + "\n\tAND e.Jenis_Kiriman = ?\n\tAND e.Kantor_Asal IN (SELECT k.Kode_Kantor FROM cch_kantor AS k WHERE k.Regional = ? OR k.Kode_Kantor = ?)" + "\nGROUP BY Code, Label\nORDER BY Count DESC, Code ASC\nLIMIT ? OFFSET ?",
			query:     complaints.BreakdownQuery{Direction: complaints.BreakdownDirectionSent, Kind: complaints.BreakdownKindProblem, From: &from, To: &to, ProductCode: "REG", RegionalCode: "4", Limit: 10, Offset: 0},
			wantArgs:  []driver.Value{from, to, "REG", "4", "4", 10, 0},
		},
		{
			name:      "received shipment with only target and region",
			statement: receivedBreakdownShipmentSQL + "\n\tAND e.Kantor_Tujuan = u.Kantor_Tujuan_Update\n\tAND u.Kantor_Tujuan_Update IN (SELECT k.Kode_Kantor FROM cch_kantor AS k WHERE k.Regional = ? OR k.Kode_Kantor = ?)" + "\nGROUP BY Code, Label\nORDER BY Count DESC, Code ASC\nLIMIT ? OFFSET ?",
			query:     complaints.BreakdownQuery{Direction: complaints.BreakdownDirectionReceived, Kind: complaints.BreakdownKindShipment, From: &from, To: &to, RegionalCode: "4", OnlyTarget: true, Limit: 10, Offset: 0},
			wantArgs:  []driver.Value{from, to, "4", "4", 10, 0},
		},
		{
			name:      "received problem product",
			statement: receivedBreakdownProblemSQL + "\n\tAND e.Jenis_Kiriman = ?" + "\nGROUP BY Code, Label\nORDER BY Count DESC, Code ASC\nLIMIT ? OFFSET ?",
			query:     complaints.BreakdownQuery{Direction: complaints.BreakdownDirectionReceived, Kind: complaints.BreakdownKindProblem, From: &from, To: &to, ProductCode: "REG", Limit: 10, Offset: 0},
			wantArgs:  []driver.Value{from, to, "REG", 10, 0},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectQuery(regexp.QuoteMeta(test.statement)).WithArgs(test.wantArgs...).
				WillReturnRows(sqlmock.NewRows([]string{"Code", "Label", "Count"}).AddRow("REG", "Reguler", 7))
			results, err := NewMySQLComplaintRepository(database).ListBreakdown(context.Background(), test.query, "40005CC")
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].Code != "REG" || results[0].Count != 7 {
				t.Fatalf("breakdown rows = %#v", results)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBreakdownRejectsInvalidQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLComplaintRepository(database)
	validFrom := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	for _, probe := range []struct {
		name  string
		query complaints.BreakdownQuery
	}{
		{name: "bad direction", query: complaints.BreakdownQuery{Direction: "sideways", Kind: complaints.BreakdownKindShipment, From: &validFrom, To: &validTo, Limit: 10}},
		{name: "bad kind", query: complaints.BreakdownQuery{Direction: complaints.BreakdownDirectionSent, Kind: "other", From: &validFrom, To: &validTo, Limit: 10}},
		{name: "missing dates", query: complaints.BreakdownQuery{Direction: complaints.BreakdownDirectionSent, Kind: complaints.BreakdownKindShipment, Limit: 10}},
	} {
		t.Run(probe.name, func(t *testing.T) {
			_, err := repository.ListBreakdown(context.Background(), probe.query, "40005CC")
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("ListBreakdown() error = %v, want invalid query", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSentOpenDurationBucketsRejectInvalidBoundsBeforeQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	validDate := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		query  complaints.SentOpenDurationQuery
		office string
	}{
		{name: "missing end date", query: complaints.SentOpenDurationQuery{From: &validDate}, office: "40005CC"},
		{name: "invalid office list", query: complaints.SentOpenDurationQuery{From: &validDate, To: &validDate}, office: "40005CC,40006CC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewMySQLComplaintRepository(database).CountSentOpenDurationBuckets(context.Background(), test.query, test.office)
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("CountSentOpenDurationBuckets() error = %v, want invalid query", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLComplaintRepositoryListsSentOpenReportWithLatestStatusAndFilters(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	createdAt := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.September, 14, 9, 30, 0, 0, time.UTC)
	statement := sentOpenReportSQL + "\n\tAND c.Jenis_Kiriman = ?\nORDER BY c.Tanggal_Tambah DESC, c.ID_Pengaduan DESC LIMIT ? OFFSET ?"
	mock.ExpectQuery(regexp.QuoteMeta(statement)).
		WithArgs("40005CC", from, to, "REG", 10, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "Kantor_Asal_Update", "Nomor_Kiriman", "Jenis_Kiriman",
			"Tanggal_Tambah", "Kantor_Tujuan", "Status_Update", "Deskripsi_Status", "Kantor_Asal_Update_Latest",
			"Tanggal_Update", "DurationSeconds",
		}).AddRow("TKT-001", "40005CC", "RESI-001", "REG", createdAt, "40007CC", "102", "RESPON", "40006CC", updatedAt, int64(176400)))

	results, err := NewMySQLComplaintRepository(database).ListSentOpenReport(context.Background(), complaints.SentOpenReportQuery{
		From: &from, To: &to, ProductCode: "REG", Limit: 10, Offset: 20,
	}, "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ComplaintID != "TKT-001" || results[0].LatestStatusCode != "102" || results[0].LatestStatusOfficeCode != "40006CC" || results[0].DurationSeconds != 176400 || results[0].OriginOfficeUpdateCode != "40005CC" {
		t.Fatalf("unexpected sent-open report rows: %#v", results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSentOpenReportRequiresDateBoundsAndRejectsInvalidOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLComplaintRepository(database)
	validFrom := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		query  complaints.SentOpenReportQuery
		office string
	}{
		{name: "missing date range", query: complaints.SentOpenReportQuery{Limit: 10}, office: "40005CC"},
		{name: "reversed date range", query: complaints.SentOpenReportQuery{From: &validTo, To: &validFrom, Limit: 10}, office: "40005CC"},
		{name: "spoofable multi-office value", query: complaints.SentOpenReportQuery{From: &validFrom, To: &validTo, Limit: 10}, office: "40005CC,40006CC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := repository.ListSentOpenReport(context.Background(), test.query, test.office)
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("ListSentOpenReport() error = %v, want invalid report query", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLComplaintRepositoryDoesNotSelectCustomerContactOrDescription(t *testing.T) {
	for _, sensitiveField := range []string{"Nama_Pelanggan", "Email_Pelanggan", "Nomor_HP_Pelanggan", "Alamat_Pelanggan", "Uraian_Pengaduan"} {
		if strings.Contains(strings.ToLower(complaintListSelectSQL), strings.ToLower(sensitiveField)) {
			t.Errorf("query unexpectedly selects %s", sensitiveField)
		}
	}
}

func TestMySQLComplaintRepositoryBuildsCallCenterOpenScope(t *testing.T) {
	query := complaints.ComplaintListQuery{
		OfficeCode:  "40005CC",
		OfficeMatch: complaints.MatchOriginOrAllTargets,
		OpenOnly:    true,
		Limit:       20,
		Offset:      40,
	}
	statement, args := buildComplaintListQuery(query)
	if !strings.Contains(statement, "Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan") ||
		!strings.Contains(statement, "Status_Akhir <> '109'") ||
		strings.Contains(statement, "OR Kantor_Tujuan = ?") ||
		strings.Contains(statement, "OR FIND_IN_SET(?, REPLACE(COALESCE(Kantor_Tembusan") {
		t.Fatalf("call center open query has unexpected scope: %s", statement)
	}
	if len(args) != 4 || args[0] != "40005CC" || args[1] != "40005CC" || args[2] != 20 || args[3] != 40 {
		t.Fatalf("query args = %#v, want office twice then limit/offset", args)
	}
}

func TestMySQLComplaintRepositoryBuildsCallCenterCompletedScope(t *testing.T) {
	query := complaints.ComplaintListQuery{
		OfficeCode:    "40005CC",
		OfficeMatch:   complaints.MatchAllRoutingFields,
		CompletedOnly: true,
		Limit:         20,
		Offset:        40,
	}
	statement, args := buildComplaintListQuery(query)
	if !strings.Contains(statement, "Status_Akhir = '109'") || !strings.Contains(statement, "Kantor_Asal = ? OR Kantor_Tujuan = ?") ||
		strings.Contains(statement, "Status_Akhir <> '109'") {
		t.Fatalf("call center completed query has unexpected scope: %s", statement)
	}
	if len(args) != 6 || args[0] != "40005CC" || args[1] != "40005CC" || args[2] != "40005CC" || args[3] != "40005CC" || args[4] != 20 || args[5] != 40 {
		t.Fatalf("query args = %#v, want all routing offices then limit/offset", args)
	}
}

func TestMySQLComplaintRepositoryBuildsReceivedAndSentOpenReportScopes(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name        string
		officeMatch complaints.ComplaintOfficeMatch
		wanted      string
		unwanted    string
	}{
		{name: "received", officeMatch: complaints.MatchAllTargetsOnly, wanted: "FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan", unwanted: "Kantor_Asal = ?"},
		{name: "sent", officeMatch: complaints.MatchOriginOnly, wanted: "Kantor_Asal = ?", unwanted: "Semua_Tujuan"},
	} {
		t.Run(test.name, func(t *testing.T) {
			statement, args := buildComplaintListQuery(complaints.ComplaintListQuery{
				OfficeCode: "40005CC", OfficeMatch: test.officeMatch, OpenOnly: true,
				From: &from, To: &to, Limit: 10, Offset: 20,
			})
			whereClause := strings.SplitN(statement, "\nWHERE ", 2)[1]
			whereClause = strings.SplitN(whereClause, "\nORDER BY", 2)[0]
			if !strings.Contains(whereClause, test.wanted) || strings.Contains(whereClause, test.unwanted) || !strings.Contains(whereClause, "Status_Akhir <> '109'") || !strings.Contains(whereClause, "Tanggal_Pengaduan BETWEEN ? AND ?") {
				t.Fatalf("%s report query has unexpected predicate: %s", test.name, statement)
			}
			if len(args) != 5 || args[0] != "40005CC" || args[1] != from || args[2] != to || args[3] != 10 || args[4] != 20 {
				t.Fatalf("%s report query args = %#v", test.name, args)
			}
		})
	}
}

func TestCompletedComplaintHistoryQueryUsesCompletedParentRoutingScope(t *testing.T) {
	for _, fragment := range []string{
		"c.Status_Akhir = '109'",
		"c.Kantor_Asal = ? OR c.Kantor_Tujuan = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Kantor_Tembusan, ''), ' ', '')) > 0",
		"ORDER BY h.Tanggal_Update ASC, h.ID_Histori ASC",
	} {
		if !strings.Contains(completedComplaintHistoryQuerySQL, fragment) {
			t.Errorf("completed history query is missing required clause %q", fragment)
		}
	}
}

func TestCompletedComplaintCountUsesDashboardRoutingPredicate(t *testing.T) {
	if !strings.Contains(completedComplaintCountSQL, "FROM cchselesai") ||
		!strings.Contains(completedComplaintCountSQL, "Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan") {
		t.Fatalf("completed count query does not match dashboard predicate: %s", completedComplaintCountSQL)
	}
}

func TestOpenComplaintCountUsesOpenDashboardPredicate(t *testing.T) {
	if !strings.Contains(openComplaintCountSQL, "FROM cchentri") ||
		!strings.Contains(openComplaintCountSQL, "Status_Akhir <> '109'") ||
		!strings.Contains(openComplaintCountSQL, "Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan") {
		t.Fatalf("open count query does not match dashboard predicate: %s", openComplaintCountSQL)
	}
}

func TestReceivedAndSentOpenCountsUseSeparateLegacyPredicates(t *testing.T) {
	if !strings.Contains(receivedOpenComplaintCountSQL, "Status_Akhir <> '109'") ||
		!strings.Contains(receivedOpenComplaintCountSQL, "FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan") ||
		strings.Contains(receivedOpenComplaintCountSQL, "Kantor_Asal") ||
		strings.Contains(receivedOpenComplaintCountSQL, "Request_Status") {
		t.Fatalf("received-open count query has unexpected scope: %s", receivedOpenComplaintCountSQL)
	}
	if !strings.Contains(sentOpenComplaintCountSQL, "Status_Akhir <> '109'") ||
		!strings.Contains(sentOpenComplaintCountSQL, "Kantor_Asal = ?") ||
		strings.Contains(sentOpenComplaintCountSQL, "Semua_Tujuan") ||
		strings.Contains(sentOpenComplaintCountSQL, "Request_Status") {
		t.Fatalf("sent-open count query has unexpected scope: %s", sentOpenComplaintCountSQL)
	}
}

func TestMySQLComplaintRepositoryCountsReceivedAndSentOpenRequestsWithDateRange(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name      string
		statement string
		count     func(*MySQLComplaintRepository, complaints.OpenCountQuery) (int64, error)
	}{
		{
			name:      "received",
			statement: receivedOpenComplaintCountSQL + " AND Tanggal_Pengaduan BETWEEN ? AND ?",
			count: func(repository *MySQLComplaintRepository, query complaints.OpenCountQuery) (int64, error) {
				return repository.CountReceivedOpenForCallCenter(context.Background(), query, "40005CC")
			},
		},
		{
			name:      "sent",
			statement: sentOpenComplaintCountSQL + " AND Tanggal_Pengaduan BETWEEN ? AND ?",
			count: func(repository *MySQLComplaintRepository, query complaints.OpenCountQuery) (int64, error) {
				return repository.CountSentOpenForCallCenter(context.Background(), query, "40005CC")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectQuery(regexp.QuoteMeta(test.statement)).
				WithArgs("40005CC", from, to).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

			count, err := test.count(NewMySQLComplaintRepository(database), complaints.OpenCountQuery{From: &from, To: &to})
			if err != nil || count != 7 {
				t.Fatalf("count = %d, error = %v, want 7 and no error", count, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestListSentSummaryGroupsByOriginOfficeWithRegionalAndProductFilters(t *testing.T) {
	from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.September, 30, 23, 59, 59, 0, time.UTC)
	for _, test := range []struct {
		name         string
		regionalCode string
		productCode  string
		extraSQL     string
		args         []driver.Value
	}{
		{name: "all regions and products", regionalCode: "0", productCode: "999"},
		{name: "single product and region", regionalCode: "4", productCode: "REG",
			extraSQL: "\n\tAND e.Jenis_Kiriman = ?\n\tAND ((k.Regional = ?) OR (k.Kode_Kantor = ?))", args: []driver.Value{"REG", "4", "4"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			statement := sentSummarySQL + test.extraSQL + "\nGROUP BY e.Kantor_Asal, k.Nama_Kantor\nORDER BY e.Kantor_Asal ASC\nLIMIT ? OFFSET ?"
			arguments := []driver.Value{from, to}
			arguments = append(arguments, test.args...)
			arguments = append(arguments, 10, 0)
			mock.ExpectQuery(regexp.QuoteMeta(statement)).WithArgs(arguments...).
				WillReturnRows(sqlmock.NewRows([]string{"OfficeCode", "OfficeName", "Total", "Completed", "Unresponded"}).
					AddRow("40005CC", "Divre A", 10, 6, 3))

			results, err := NewMySQLComplaintRepository(database).ListSentSummary(context.Background(), complaints.SentSummaryQuery{
				From: &from, To: &to, ProductCode: test.productCode, RegionalCode: test.regionalCode, Limit: 10, Offset: 0,
			}, "40005CC")
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].OfficeCode != "40005CC" || results[0].Total != 10 || results[0].Completed != 6 || results[0].Open != 4 || results[0].Responded != 7 || results[0].Unresponded != 3 {
				t.Fatalf("unexpected sent summary rows: %#v", results)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSentSummaryRejectsInvalidRegionalProductOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLComplaintRepository(database)
	validFrom := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		query  complaints.SentSummaryQuery
		office string
	}{
		{name: "missing date range", query: complaints.SentSummaryQuery{Limit: 10}, office: "40005CC"},
		{name: "region list injection", query: complaints.SentSummaryQuery{From: &validFrom, To: &validTo, RegionalCode: "4,5", Limit: 10}, office: "40005CC"},
		{name: "multi-office spoof", query: complaints.SentSummaryQuery{From: &validFrom, To: &validTo, Limit: 10}, office: "40005CC,40006CC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := repository.ListSentSummary(context.Background(), test.query, test.office)
			if !errors.Is(err, complaints.ErrInvalidComplaintListQuery) {
				t.Fatalf("ListSentSummary() error = %v, want invalid query", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLComplaintRepositoryRequiresDatabase(t *testing.T) {
	var repository *MySQLComplaintRepository
	_, err := repository.ListInvolvingOffice(context.Background(), complaints.ComplaintListQuery{OfficeCode: "40005CC"})
	if !errors.Is(err, ErrRepositoryNotConfigured) {
		t.Fatalf("ListInvolvingOffice() error = %v, want repository not configured", err)
	}
}
