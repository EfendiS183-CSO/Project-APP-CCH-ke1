package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/customers"
)

func TestMySQLCustomerRepositorySearchesProfilesInOneOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(callCenterCustomerSearchQuery)).
		WithArgs("40005CC", "cus%", "cus%", 5).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan", "Nama_Lengkap", "Email", "Nomor_HP", "Alamat"}).
			AddRow(10, "CUSTOMER TEST", "customer@example.test", "0812345678", nil))

	profiles, err := NewMySQLCustomerRepository(database).SearchCallCenterProfiles(context.Background(), customers.SearchQuery{
		Term:       "cus",
		OfficeCode: "40005CC",
		Limit:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ID != 10 || profiles[0].FullName != "CUSTOMER TEST" || profiles[0].Phone != "0812345678" || profiles[0].Address != "" {
		t.Fatalf("unexpected customer search result: %#v", profiles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerSearchEscapesLikeWildcardsAndEnforcesOfficeScope(t *testing.T) {
	pattern := escapeSearchPrefix("a%_!")
	if pattern != "a!%!_!!%" {
		t.Fatalf("escaped prefix = %q, want literal wildcard escaping", pattern)
	}
	for _, fragment := range []string{
		"WHERE Kantor_Tambah = ?",
		"Nama_Lengkap LIKE ? ESCAPE '!' OR Nomor_HP LIKE ? ESCAPE '!'",
		"LIMIT ?",
	} {
		if !strings.Contains(callCenterCustomerSearchQuery, fragment) {
			t.Errorf("customer search query is missing %q", fragment)
		}
	}
}

func TestMySQLCustomerRepositoryLoadsTicketProfileByIDAndOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(callCenterTicketProfileQuery)).
		WithArgs(int64(42), "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan", "Nama_Lengkap", "Email", "Nomor_HP", "Alamat"}).
			AddRow(42, "CUSTOMER CANONICAL", "canonical@example.test", "0812345678", "CANONICAL ADDRESS"))

	profile, err := NewMySQLCustomerRepository(database).GetCallCenterProfileForOffice(context.Background(), 42, "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != 42 || profile.FullName != "CUSTOMER CANONICAL" || profile.Email != "canonical@example.test" || profile.Address != "CANONICAL ADDRESS" {
		t.Fatalf("unexpected canonical profile: %#v", profile)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerTicketProfileQueryRequiresExactProfileIDAndOffice(t *testing.T) {
	for _, fragment := range []string{
		"WHERE ID_Pelanggan = ? AND Kantor_Tambah = ?",
		"LIMIT 1",
	} {
		if !strings.Contains(callCenterTicketProfileQuery, fragment) {
			t.Errorf("ticket profile query is missing %q", fragment)
		}
	}
}
