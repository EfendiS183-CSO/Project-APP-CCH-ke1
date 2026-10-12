package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/complaints"
)

func TestMySQLComplaintRepositoryListsHistoryWithinOpenCallCenterParentScope(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	updatedAt := time.Date(2026, time.September, 29, 8, 30, 0, 0, time.UTC)
	query := complaints.ComplaintHistoryQuery{
		ComplaintID: "TKT-001",
		OfficeCode:  "40005CC",
		Limit:       10,
		Offset:      20,
	}
	mock.ExpectQuery(regexp.QuoteMeta(complaintHistoryQuerySQL)).
		WithArgs("TKT-001", "40005CC", "40005CC", 10, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "ID_Histori", "Status_Update", "Deskripsi_Status", "Kantor_Asal_Update",
			"Deskripsi_Update", "Tanggal_Update", "Petugas_Update", "Durasi_Antar_Status",
		}).AddRow("TKT-001", "HIST-002", "102", "RESPON", "40006CC", "Sedang ditangani", updatedAt, "cs.agent", int64(3600)))

	entries, err := NewMySQLComplaintRepository(database).ListOpenTicketHistory(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.ComplaintID != "TKT-001" || entry.HistoryID != "HIST-002" || entry.StatusCode != "102" || entry.StatusDescription != "RESPON" {
		t.Fatalf("unexpected history entry: %#v", entry)
	}
	if entry.OriginOfficeCode != "40006CC" || entry.Description != "Sedang ditangani" || entry.UpdatedBy != "cs.agent" || !entry.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected history fields: %#v", entry)
	}
	if entry.DurationSeconds == nil || *entry.DurationSeconds != 3600 {
		t.Fatalf("duration = %v, want 3600 seconds", entry.DurationSeconds)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLComplaintRepositoryRejectsInvalidHistoryQueryBeforeDatabaseAccess(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	_, err = NewMySQLComplaintRepository(database).ListOpenTicketHistory(context.Background(), complaints.ComplaintHistoryQuery{
		ComplaintID: "TKT-001",
		OfficeCode:  "",
	})
	if !errors.Is(err, complaints.ErrInvalidComplaintHistoryQuery) {
		t.Fatalf("ListOpenTicketHistory() error = %v, want invalid history query", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintHistoryQueryConstrainsOpenParentAndExactOfficeMembership(t *testing.T) {
	for _, fragment := range []string{
		"INNER JOIN cchentri AS c ON c.ID_Pengaduan = h.ID_Pengaduan",
		"c.Status_Akhir <> '109'",
		"c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0",
		"ORDER BY h.Tanggal_Update ASC, h.ID_Histori ASC",
	} {
		if !strings.Contains(complaintHistoryQuerySQL, fragment) {
			t.Errorf("history query is missing required clause %q", fragment)
		}
	}
	for _, forbiddenField := range []string{"Nama_Pelanggan", "Email_Pelanggan", "Nomor_HP_Pelanggan", "Alamat_Pelanggan"} {
		if strings.Contains(strings.ToLower(complaintHistoryQuerySQL), strings.ToLower(forbiddenField)) {
			t.Errorf("history query unexpectedly selects %s", forbiddenField)
		}
	}
}

func TestMySQLComplaintRepositoryListsOnlyActiveCallCenterOriginOptions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(callCenterStatusOptionsQuerySQL)).
		WithArgs("TKT-001", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"ID_Status", "Deskripsi_Status"}).
			AddRow("103", "KONFIRMASI").
			AddRow("109", "SELESAI"))
	options, err := NewMySQLComplaintRepository(database).ListCallCenterStatusOptions(context.Background(), "TKT-001", "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].Code != "103" || options[1].Code != "109" {
		t.Fatalf("unexpected status options: %#v", options)
	}
	for _, fragment := range []string{"c.Kantor_Asal = ?", "c.Status_Akhir <> '109'", "s.Aktif = 'Y'", "s.ID_Status IN ('103', '109')"} {
		if !strings.Contains(callCenterStatusOptionsQuerySQL, fragment) {
			t.Errorf("status option SQL missing %q", fragment)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLComplaintRepositoryHidesClosedOrOtherOfficeStatusOptions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(callCenterStatusOptionsQuerySQL)).
		WithArgs("TKT-OUTSIDE", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"ID_Status", "Deskripsi_Status"}))
	options, err := NewMySQLComplaintRepository(database).ListCallCenterStatusOptions(context.Background(), "TKT-OUTSIDE", "40005CC")
	if !errors.Is(err, sql.ErrNoRows) || options != nil {
		t.Fatalf("out-of-scope result=%#v error=%v, want no rows", options, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
