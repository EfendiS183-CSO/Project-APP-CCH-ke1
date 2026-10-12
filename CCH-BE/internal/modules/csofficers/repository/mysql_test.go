package repository

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/csofficers"
)

func TestMySQLRepositoryListsFilteredCSOfficersAndScansNullableFields(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	query := csofficers.ListQuery{Regional: "40005", Office: "40005CC", Status: "Login", Search: "A%_!", Limit: 10, Offset: 20}
	pattern := "A!%!_!!%"
	args := []driver.Value{"40005", "40005", "40005CC", "40005CC", "Login", "Login", "A%_!", pattern, pattern}
	mock.ExpectQuery(regexp.QuoteMeta(countCSOfficersSQL)).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	columns := []string{"Username", "Regional", "Kantor", "Nama_Lengkap", "Status_Sekarang", "Email", "Phone", "Login_Terakhir", "Logout_Terakhir"}
	lastLogin := time.Date(2026, time.October, 6, 8, 30, 0, 0, time.UTC)
	listArgs := append(append([]driver.Value{}, args...), int64(10), int64(20))
	mock.ExpectQuery(regexp.QuoteMeta(listCSOfficersSQL)).WithArgs(listArgs...).WillReturnRows(sqlmock.NewRows(columns).AddRow(
		"cs.user", "40005", "40005CC", "CS User", "Login", "cs@example.test", nil, lastLogin, nil))
	repository := NewMySQLRepository(database)
	page, err := repository.List(context.Background(), query)
	if err != nil || page.Total != 1 || len(page.Data) != 1 || page.Data[0].Username != "cs.user" || page.Data[0].Phone != nil || page.Data[0].LastLogout != nil || page.Data[0].LastLogin == nil || page.Limit != 10 || page.Offset != 20 {
		t.Fatalf("List() = %#v, error = %v", page, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryGetsOnlyVisibleCSOfficerAndHandlesNotFound(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	columns := []string{"Username", "Regional", "Kantor", "Nama_Lengkap", "Status_Sekarang", "Email", "Phone", "Login_Terakhir", "Logout_Terakhir"}
	lastLogout := time.Date(2026, time.October, 5, 16, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(getCSOfficerSQL)).WithArgs("cs.user").WillReturnRows(sqlmock.NewRows(columns).AddRow(
		"cs.user", nil, nil, "CS User", "Logout", nil, nil, nil, lastLogout))
	mock.ExpectQuery(regexp.QuoteMeta(getCSOfficerSQL)).WithArgs("hidden.user").WillReturnRows(sqlmock.NewRows(columns))
	repository := NewMySQLRepository(database)
	value, err := repository.Get(context.Background(), "cs.user")
	if err != nil || value.Username != "cs.user" || value.Regional != nil || value.Email != nil || value.LastLogout == nil {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if _, err := repository.Get(context.Background(), "hidden.user"); err == nil {
		t.Fatal("missing CS officer should return scan error for service mapping")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
