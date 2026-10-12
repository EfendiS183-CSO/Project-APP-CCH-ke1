package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/complaints"
)

func TestMySQLComplaintRepositoryGetsOpenDetailWithinExactOfficeScope(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	complaintDate := time.Date(2026, time.September, 28, 9, 15, 0, 0, time.UTC)
	statusDate := time.Date(2026, time.September, 29, 8, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(complaintDetailQuerySQL)).
		WithArgs("TKT-001", "40005CC", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "Nama_Pelanggan", "Email_Pelanggan", "Nomor_HP_Pelanggan", "Alamat_Pelanggan",
			"Kantor_Asal", "Kantor_Tujuan", "Kantor_Tembusan", "Semua_Tujuan", "Jenis_Pengaduan", "Sumber_Pengaduan",
			"Jenis_Kiriman", "Nomor_Kiriman", "Tanggal_Pengaduan", "Kantor_Asal_Kiriman", "Kantor_Tujuan_Kiriman",
			"Ringkasan_Pengaduan", "Uraian_Pengaduan", "Status_Akhir", "Deskripsi_Status", "Tanggal_Status", "Kantor_Status", "Request_Status",
		}).AddRow("TKT-001", "Nama Pelanggan", "customer@example.test", "0800000000", "Alamat", "40005CC", "40006CC", "40007CC", "40006CC,40005CC", int64(2), int64(1), "PKG", "RESI-001", complaintDate, "40005CC", "40006CC", "Ringkasan", "Uraian", "102", "RESPON", statusDate, "40006CC", "0"))

	detail, err := NewMySQLComplaintRepository(database).GetOpenTicketForOffice(context.Background(), complaints.ComplaintDetailQuery{ComplaintID: "TKT-001", OfficeCode: "40005CC"})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ComplaintID != "TKT-001" || detail.CustomerName != "Nama Pelanggan" || detail.CustomerEmail != "customer@example.test" || detail.CustomerPhone != "0800000000" {
		t.Fatalf("customer fields = %#v", detail)
	}
	if detail.StatusCode != "102" || detail.StatusDescription != "RESPON" || detail.StatusOfficeCode != "40006CC" || detail.RequestStatus != "0" {
		t.Fatalf("status fields = %#v", detail)
	}
	if detail.ComplaintDate == nil || !detail.ComplaintDate.Equal(complaintDate) || !detail.StatusDate.Equal(statusDate) {
		t.Fatalf("date fields = %#v", detail)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintDetailQueryRequiresOpenExactOfficeScopeAndBoundedProjection(t *testing.T) {
	for _, fragment := range []string{
		"WHERE c.ID_Pengaduan = ?",
		"c.Status_Akhir <> '109'",
		"c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0",
		"LIMIT 1",
	} {
		if !strings.Contains(complaintDetailQuerySQL, fragment) {
			t.Errorf("detail query is missing required clause %q", fragment)
		}
	}
	for _, internalField := range []string{"ID_Pelanggan", "Already_Validated", "Diubah_Oleh"} {
		if strings.Contains(complaintDetailQuerySQL, internalField) {
			t.Errorf("detail query unexpectedly selects internal field %s", internalField)
		}
	}
}

func TestCompletedComplaintDetailQueryRequiresClosedRoutingScope(t *testing.T) {
	for _, fragment := range []string{
		"WHERE c.ID_Pengaduan = ?",
		"c.Status_Akhir = '109'",
		"c.Kantor_Asal = ? OR c.Kantor_Tujuan = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Kantor_Tembusan, ''), ' ', '')) > 0",
		"LIMIT 1",
	} {
		if !strings.Contains(completedComplaintDetailQuerySQL, fragment) {
			t.Errorf("completed detail query is missing required clause %q", fragment)
		}
	}
}

func TestMySQLComplaintRepositoryRejectsInvalidDetailBeforeQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	_, err = NewMySQLComplaintRepository(database).GetOpenTicketForOffice(context.Background(), complaints.ComplaintDetailQuery{ComplaintID: "TKT-001"})
	if !errors.Is(err, complaints.ErrInvalidComplaintDetailQuery) {
		t.Fatalf("GetOpenTicketForOffice() error = %v, want invalid detail query", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
