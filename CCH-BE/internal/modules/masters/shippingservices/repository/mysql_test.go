package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/shippingservices"
)

func TestMySQLRepositoryListsAndGetsShippingServices(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(listShippingServicesSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Layanan", "Deskripsi", "URL_Lacak", "Waktu_Akhir", "Status_Aktif"}).
			AddRow("REG", "Reguler", "https://track.example/{item}", updatedAt, "Y").
			AddRow("EXP", "Express", "", nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(getShippingServiceSQL)).WithArgs("REG").
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Layanan", "Deskripsi", "URL_Lacak", "Waktu_Akhir", "Status_Aktif"}).
			AddRow("REG", "Reguler", "https://track.example/{item}", updatedAt, "Y"))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].Code != "REG" || !values[0].Active || values[0].UpdatedAt == nil || values[1].Active || values[1].UpdatedAt != nil {
		t.Fatalf("unexpected shipping services: %#v", values)
	}
	value, err := repository.Get(context.Background(), "REG")
	if err != nil || value.Description != "Reguler" || value.TrackingURL != "https://track.example/{item}" {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryInsertsAndMapsDuplicateCode(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	service := shippingservices.ShippingService{Code: "REG", Description: "Reguler", TrackingURL: "https://track.example/{item}", Active: true, UpdatedAt: &updatedAt}
	mock.ExpectExec(regexp.QuoteMeta(insertShippingServiceSQL)).
		WithArgs("REG", "Reguler", "https://track.example/{item}", &updatedAt, "Y").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertShippingServiceSQL)).
		WithArgs("REG", "Reguler", "https://track.example/{item}", &updatedAt, "Y").
		WillReturnError(&mysqldriver.MySQLError{Number: 1062})

	repository := NewMySQLRepository(database)
	if err := repository.Insert(context.Background(), service); err != nil {
		t.Fatal(err)
	}
	if err := repository.Insert(context.Background(), service); !errors.Is(err, shippingservices.ErrAlreadyExists) {
		t.Fatalf("duplicate Insert() error = %v, want duplicate code", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdatesAndDeactivatesShippingService(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 11, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(updateShippingServiceSQL)).
		WithArgs("Reguler Baru", "https://track.example/{item}", &updatedAt, "N", "REG").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateShippingServiceSQL)).
		WithArgs(updatedAt, "REG").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateShippingServiceSQL)).
		WithArgs(updatedAt, "REG").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(shippingServiceExistsSQL)).WithArgs("REG").
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Layanan"}).AddRow("REG"))

	repository := NewMySQLRepository(database)
	if err := repository.Update(context.Background(), shippingservices.ShippingService{Code: "REG", Description: "Reguler Baru", TrackingURL: "https://track.example/{item}", Active: false, UpdatedAt: &updatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), "REG", updatedAt); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), "REG", updatedAt); err != nil {
		t.Fatalf("repeated deactivation should be idempotent: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
