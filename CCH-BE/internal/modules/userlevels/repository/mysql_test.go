package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/userlevels"
)

func TestMySQLRepositoryListsAndGetsUserLevels(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(countUserLevelsSQL)).WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta(listUserLevelsSQL)).WithArgs(2, 1).WillReturnRows(sqlmock.NewRows([]string{"User_Level_ID", "User_Level_Name"}).
		AddRow(2, "Call Center").AddRow(3, "Admin UPT"))
	mock.ExpectQuery(regexp.QuoteMeta(getUserLevelSQL)).WithArgs(int64(-2)).WillReturnRows(sqlmock.NewRows([]string{"User_Level_ID", "User_Level_Name"}).AddRow(-2, "Anonymous"))

	repository := NewMySQLRepository(database)
	page, err := repository.List(context.Background(), userlevels.ListQuery{Limit: 2, Offset: 1})
	if err != nil || page.Total != 3 || len(page.Data) != 2 || page.Data[0].ID != 2 || page.Limit != 2 || page.Offset != 1 {
		t.Fatalf("List() = %#v, error = %v", page, err)
	}
	value, err := repository.Get(context.Background(), -2)
	if err != nil || value.ID != -2 || value.Name != "Anonymous" {
		t.Fatalf("Get(-2) = %#v, error = %v", value, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryListUsesOnlyRoleCatalogColumns(t *testing.T) {
	if regexp.MustCompile(`(?i)password|permission`).MatchString(listUserLevelsSQL) {
		t.Fatalf("user-level catalog query should not read credential/permission data: %s", listUserLevelsSQL)
	}
	if regexp.MustCompile(`(?i)password|permission`).MatchString(getUserLevelSQL) {
		t.Fatalf("user-level detail query should not read credential/permission data: %s", getUserLevelSQL)
	}
}
