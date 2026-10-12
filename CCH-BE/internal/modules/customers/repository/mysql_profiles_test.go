package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/customers"
)

func TestMySQLRepositoryListsGetsAndCreatesOfficeScopedProfiles(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	columns := []string{"ID_Pelanggan", "Nama_Lengkap", "Email", "Nomor_HP", "Alamat"}
	mock.ExpectQuery(regexp.QuoteMeta(countProfilesQuery)).WithArgs("40005CC").WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(listProfilesQuery)).WithArgs("40005CC", 10, 20).WillReturnRows(sqlmock.NewRows(columns).AddRow(13, "ADA", "ada@example.test", "+62123", nil))
	mock.ExpectQuery(regexp.QuoteMeta(getProfileQuery)).WithArgs(int64(13), "40005CC").WillReturnRows(sqlmock.NewRows(columns).AddRow(13, "ADA", "ada@example.test", "+62123", nil))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(duplicateProfileNameQuery)).WithArgs("NEW NAME").WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan"}))
	mock.ExpectExec(regexp.QuoteMeta(insertProfileQuery)).WithArgs("NEW NAME", "new@example.test", "+62123", "ADDRESS", "40005CC", "manager", "40005CC", "manager").WillReturnResult(sqlmock.NewResult(22, 1))
	mock.ExpectCommit()
	repository := NewMySQLCustomerRepository(database)
	query := customers.ProfileListQuery{OfficeCode: "40005CC", Limit: 10, Offset: 20}
	profiles, err := repository.ListProfiles(context.Background(), query)
	if err != nil || len(profiles.Data) != 1 || profiles.Data[0].ID != 13 || profiles.Total != 1 || profiles.Limit != 10 || profiles.Offset != 20 {
		t.Fatalf("ListProfiles() = %#v, error = %v", profiles, err)
	}
	profile, err := repository.GetProfile(context.Background(), 13, "40005CC")
	if err != nil || profile.Address != "" {
		t.Fatalf("GetProfile() = %#v, error = %v", profile, err)
	}
	created, err := repository.CreateProfile(context.Background(), customers.Profile{FullName: "NEW NAME", Email: "new@example.test", Phone: "+62123", Address: "ADDRESS"}, "40005CC", "manager")
	if err != nil || created.ID != 22 {
		t.Fatalf("CreateProfile() = %#v, error = %v", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdatesAndProtectsReferencedProfileDeletion(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockProfileQuery)).WithArgs(int64(13), "40005CC").WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan"}).AddRow(13))
	mock.ExpectExec(regexp.QuoteMeta(updateProfileQuery)).WithArgs("ADA", "ada@example.test", "+62123", "ADDRESS", "40005CC", "manager", int64(13), "40005CC").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockProfileQuery)).WithArgs(int64(13), "40005CC").WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan"}).AddRow(13))
	mock.ExpectQuery(regexp.QuoteMeta(ticketProfileReferenceQuery)).WithArgs(int64(13)).WillReturnRows(sqlmock.NewRows([]string{"ID_Pengaduan"}).AddRow("TICKET-1"))
	mock.ExpectRollback()
	repository := NewMySQLCustomerRepository(database)
	updated, err := repository.UpdateProfile(context.Background(), 13, "40005CC", customers.Profile{FullName: "ADA", Email: "ada@example.test", Phone: "+62123", Address: "ADDRESS"}, "40005CC", "manager")
	if err != nil || updated.ID != 13 {
		t.Fatalf("UpdateProfile() = %#v, error = %v", updated, err)
	}
	if err := repository.DeleteProfile(context.Background(), 13, "40005CC"); err != customers.ErrProfileInUse {
		t.Fatalf("DeleteProfile() error = %v, want ErrProfileInUse", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
