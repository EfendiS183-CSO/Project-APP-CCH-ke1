package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/followups"
)

func TestMySQLRepositoryListsFollowUpsAcrossRelatedOfficesForVisibleTicket(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	plannedAt := time.Date(2026, time.September, 30, 13, 0, 0, 0, time.UTC)
	query := followups.Query{ComplaintID: "TKT-001", OfficeCode: "40005CC", Limit: 10, Offset: 20}
	mock.ExpectQuery(regexp.QuoteMeta(followUpListQuerySQL)).
		WithArgs("TKT-001", 10, 20, "TKT-001", "40005CC", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{
			"Parent_ID_Pengaduan", "FollowUp_ID_Pengaduan", "Kantor", "Status", "Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Keterangan",
		}).AddRow("TKT-001", "TKT-001", "40006CC", "102", plannedAt, "Rencana follow-up", nil, nil))

	entries, err := NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.ComplaintID != "TKT-001" || entry.OfficeCode != "40006CC" || entry.StatusCode != "102" || entry.PlanNotes != "Rencana follow-up" {
		t.Fatalf("unexpected follow-up entry: %#v", entry)
	}
	if entry.PlannedAt == nil || !entry.PlannedAt.Equal(plannedAt) || entry.RealizedAt != nil || entry.Outcome != "" {
		t.Fatalf("unexpected follow-up dates/outcome: %#v", entry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryListsOnlyTheTrustedCallCenterOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	plannedAt := time.Date(2026, time.September, 30, 13, 0, 0, 0, time.UTC)
	query := followups.OfficeQuery{OfficeCode: "40005CC", Limit: 10, Offset: 20}
	mock.ExpectQuery(regexp.QuoteMeta(callCenterOfficeListQuerySQL)).
		WithArgs("40005CC", 10, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "Kantor", "Status", "Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Keterangan",
		}).AddRow("TKT-001", "40005CC", "102", plannedAt, "Hubungi pelanggan", nil, nil))

	entries, err := NewMySQLRepository(database).ListForCallCenterOffice(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.ComplaintID != "TKT-001" || entry.OfficeCode != "40005CC" || entry.StatusCode != "102" || entry.PlanNotes != "Hubungi pelanggan" {
		t.Fatalf("unexpected follow-up entry: %#v", entry)
	}
	if entry.PlannedAt == nil || !entry.PlannedAt.Equal(plannedAt) || entry.RealizedAt != nil {
		t.Fatalf("unexpected follow-up dates: %#v", entry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryListsDueTodayRowsFromLegacyViewForOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	plannedAt := time.Date(2026, time.September, 30, 9, 0, 0, 0, time.UTC)
	query := followups.OfficeQuery{OfficeCode: "40005CC", Limit: 10, Offset: 5}
	mock.ExpectQuery(regexp.QuoteMeta(dueTodayOfficeListQuerySQL)).
		WithArgs("40005CC", 10, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "Kantor", "Status", "Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Keterangan",
		}).AddRow("TKT-002", "40005CC", "102", plannedAt, "Hubungi pelanggan", nil, nil))

	entries, err := NewMySQLRepository(database).ListDueTodayForCallCenterOffice(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ComplaintID != "TKT-002" || entries[0].OfficeCode != "40005CC" {
		t.Fatalf("unexpected due-today entries: %#v", entries)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReturnsEmptyListForVisibleTicketWithoutFollowUps(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(followUpListQuerySQL)).
		WithArgs("TKT-001", followups.DefaultPageSize, 0, "TKT-001", "40005CC", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{
			"Parent_ID_Pengaduan", "FollowUp_ID_Pengaduan", "Kantor", "Status", "Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Keterangan",
		}).AddRow("TKT-001", nil, nil, nil, nil, nil, nil, nil))

	entries, err := NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), followups.Query{ComplaintID: "TKT-001", OfficeCode: "40005CC"})
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries = %#v, error = %v; want empty list for visible ticket", entries, err)
	}
}

func TestMySQLFollowUpQueryScopesParentAndDoesNotFilterRelatedOfficeRows(t *testing.T) {
	for _, fragment := range []string{
		"LEFT JOIN (",
		"FROM cchfual",
		"WHERE c.ID_Pengaduan = ?",
		"c.Status_Akhir <> '109'",
		"c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0",
	} {
		if !strings.Contains(followUpListQuerySQL, fragment) {
			t.Errorf("follow-up query is missing %q", fragment)
		}
	}
	if strings.Contains(followUpListQuerySQL, "f.Kantor = ?") {
		t.Fatal("related follow-ups must not be filtered to only the caller's office")
	}
}

func TestMySQLCallCenterOfficeFollowUpQueryUsesExactOfficeAndBoundedPagination(t *testing.T) {
	for _, fragment := range []string{
		"FROM cchfu",
		"WHERE Kantor = ?",
		"LIMIT ? OFFSET ?",
	} {
		if !strings.Contains(callCenterOfficeListQuerySQL, fragment) {
			t.Errorf("Call Center office follow-up query is missing %q", fragment)
		}
	}
	if strings.Contains(callCenterOfficeListQuerySQL, "LIKE") || strings.Contains(callCenterOfficeListQuerySQL, "FIND_IN_SET") {
		t.Fatal("office follow-up query must not use partial or routed-office matching")
	}
}

func TestDueTodayOfficeQueryUsesLegacyViewAndExactOfficeFilter(t *testing.T) {
	for _, fragment := range []string{
		"FROM cchfuhi",
		"WHERE Kantor = ?",
		"ORDER BY Tanggal_Rencana ASC",
		"LIMIT ? OFFSET ?",
	} {
		if !strings.Contains(dueTodayOfficeListQuerySQL, fragment) {
			t.Errorf("due-today query is missing %q", fragment)
		}
	}
	if strings.Contains(dueTodayOfficeListQuerySQL, "LIKE") || strings.Contains(dueTodayOfficeListQuerySQL, "FIND_IN_SET") {
		t.Fatal("due-today office query must use exact office matching")
	}
}

func TestMySQLRepositoryRejectsInvalidQueryAndNilDatabase(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), followups.Query{})
	if !errors.Is(err, followups.ErrInvalidQuery) {
		t.Fatalf("invalid query error = %v, want invalid query", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	var repository *MySQLRepository
	_, err = repository.ListForOpenCallCenterTicket(context.Background(), followups.Query{ComplaintID: "TKT-001", OfficeCode: "40005CC"})
	if !errors.Is(err, ErrRepositoryNotConfigured) {
		t.Fatalf("nil repository error = %v, want not configured", err)
	}
}
