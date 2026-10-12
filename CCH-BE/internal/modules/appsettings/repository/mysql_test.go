package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/appsettings"
)

var settingsColumns = []string{
	"Option_ID", "Option_Default", "Show_Announcement", "Use_Announcement_Table",
	"Maintenance_Mode", "Maintenance_Finish_DateTime", "Auto_Normal_After_Maintenance",
}

func TestMySQLRepositoryReadsWritesSettings(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	finish := time.Date(2026, time.October, 20, 12, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(listSettingsSQL)).WillReturnRows(sqlmock.NewRows(settingsColumns).AddRow(
		1, "Y", "Y", "N", "Y", finish, "Y"))
	mock.ExpectQuery(regexp.QuoteMeta(getSettingsSQL)).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(settingsColumns).AddRow(
		1, "Y", "Y", "N", "Y", finish, "Y"))
	// Insert non-default row, so checkOtherDefault skips the query
	mock.ExpectExec(regexp.QuoteMeta(insertSettingsSQL)).WithArgs(
		"N", "N", "N", "N", nil, "N").WillReturnResult(sqlmock.NewResult(2, 1))
	// Update row 2
	mock.ExpectExec(regexp.QuoteMeta(updateSettingsSQL)).WithArgs(
		"Y", "Y", "Y", finish, "Y", int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	// Delete row 2 is permitted at repository level? Service blocks it, repository handles missing.
	mock.ExpectExec(regexp.QuoteMeta(deleteSettingsSQL)).WithArgs(int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 1 || !values[0].IsDefaultOption || !values[0].ShowAnnouncement || values[0].MaintenanceFinishDateTime == nil {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), 1)
	if err != nil || value.OptionID != 1 || !value.MaintenanceMode {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := repository.Insert(context.Background(), appsettings.Settings{OptionID: 2}); err != nil {
		t.Fatal(err)
	}
	updated := appsettings.Settings{OptionID: 2, ShowAnnouncement: true, UseAnnouncementTable: true, MaintenanceMode: true, MaintenanceFinishDateTime: &finish, AutoNormalAfterMaintenance: true}
	if err := repository.Update(context.Background(), updated); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsSecondDefault(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(otherDefaultSettingsSQL)).WithArgs(int64(0)).WillReturnRows(sqlmock.NewRows([]string{"Option_ID"}).AddRow(1))
	repository := NewMySQLRepository(database)
	if err := repository.Insert(context.Background(), appsettings.Settings{IsDefaultOption: true}); err != appsettings.ErrOnlyOneDefault {
		t.Fatalf("second default insert error = %v, want only one default", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
