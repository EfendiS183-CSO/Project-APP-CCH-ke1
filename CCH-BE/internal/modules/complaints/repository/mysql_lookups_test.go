package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLRepositoryListsOnlyActiveComplaintTypesAndSources(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	mock.ExpectQuery(regexp.QuoteMeta(activeComplaintTypesQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Jenis", "Deskripsi"}).
			AddRow(1, "Paket"))
	mock.ExpectQuery(regexp.QuoteMeta(activeComplaintSourcesQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"ID", "Sumber"}).
			AddRow(2, "Live Chat"))
	mock.ExpectQuery(regexp.QuoteMeta(destinationOfficesQuery)).
		WithArgs("40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Nama_Kantor"}).
			AddRow("40006CC", "Kantor Tujuan"))
	mock.ExpectQuery(regexp.QuoteMeta(copyOfficesQuery)).
		WithArgs("40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Nama_Kantor"}).
			AddRow("40007CC", "Kantor Tembusan"))
	mock.ExpectQuery(regexp.QuoteMeta(shippingServicesQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Layanan", "Deskripsi"}).
			AddRow("REG", "Reguler"))
	mock.ExpectQuery(regexp.QuoteMeta(shippingOfficesQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Nama_Kantor"}).
			AddRow("40008CC", "Kantor Kiriman"))

	repository := NewMySQLComplaintRepository(database)
	types, err := repository.ListActiveComplaintTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sources, err := repository.ListActiveComplaintSources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	destinations, err := repository.ListDestinationOffices(context.Background(), "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	copies, err := repository.ListCopyOffices(context.Background(), "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	services, err := repository.ListShippingServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	shippingOffices, err := repository.ListShippingOffices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 1 || types[0].Code != 1 || types[0].Description != "Paket" {
		t.Fatalf("unexpected complaint types: %#v", types)
	}
	if len(sources) != 1 || sources[0].ID != 2 || sources[0].Name != "Live Chat" {
		t.Fatalf("unexpected complaint sources: %#v", sources)
	}
	if len(destinations) != 1 || destinations[0].Code != "40006CC" || len(copies) != 1 || copies[0].Code != "40007CC" {
		t.Fatalf("unexpected office lookups: destinations=%#v copies=%#v", destinations, copies)
	}
	if len(services) != 1 || services[0].Code != "REG" || len(shippingOffices) != 1 || shippingOffices[0].Code != "40008CC" {
		t.Fatalf("unexpected shipment lookups: services=%#v offices=%#v", services, shippingOffices)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintLookupQueriesFilterActiveRows(t *testing.T) {
	for _, test := range []struct {
		query    string
		fragment string
	}{
		{query: activeComplaintTypesQuery, fragment: "WHERE Status_Aktif = 'Y'"},
		{query: activeComplaintSourcesQuery, fragment: "WHERE Apakah_Aktif = 'Y'"},
		{query: destinationOfficesQuery, fragment: "Sebagai_Tujuan = 'Y'"},
		{query: destinationOfficesQuery, fragment: "Apakah_Aktif = 'Y'"},
		{query: destinationOfficesQuery, fragment: "Kode_Kantor <> ?"},
		{query: copyOfficesQuery, fragment: "Sebagai_Tembusan = 'Y'"},
		{query: copyOfficesQuery, fragment: "Apakah_Aktif = 'Y'"},
		{query: copyOfficesQuery, fragment: "Kode_Kantor NOT IN ('40005CC', '40005PI', ?)"},
		{query: shippingServicesQuery, fragment: "FROM cch_jenis_layanan"},
		{query: shippingOfficesQuery, fragment: "FROM cch_kantor"},
	} {
		if !regexp.MustCompile(regexp.QuoteMeta(test.fragment)).MatchString(test.query) {
			t.Errorf("lookup query %q missing active filter %q", test.query, test.fragment)
		}
	}
}
