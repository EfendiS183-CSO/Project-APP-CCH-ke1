package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/intake/submission"
)

func TestValidateReferencesMatchesLegacyActiveAndRoutingRules(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	checks := []struct {
		query string
		args  []driver.Value
	}{
		{query: "SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Apakah_Aktif = 'Y'", args: []driver.Value{"40005CC"}},
		{query: "SELECT 1 FROM cch_jenis_pengaduan WHERE Kode_Jenis = ? AND Status_Aktif = 'Y'", args: []driver.Value{int64(1)}},
		{query: "SELECT 1 FROM cch_jenis_layanan WHERE Kode_Layanan = ?", args: []driver.Value{"REG"}},
		{query: "SELECT 1 FROM cch_sumber_pengaduan WHERE ID = ? AND Apakah_Aktif = 'Y'", args: []driver.Value{int64(7)}},
		{query: "SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Apakah_Aktif = 'Y'", args: []driver.Value{"40007CC"}},
		{query: "SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Apakah_Aktif = 'Y'", args: []driver.Value{"40008CC"}},
		{query: "SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Sebagai_Tujuan = 'Y' AND Apakah_Aktif = 'Y' AND Kode_Kantor <> ?", args: []driver.Value{"40008CC", "40005CC"}},
	}
	for _, check := range checks {
		mock.ExpectQuery(regexp.QuoteMeta(check.query)).WithArgs(check.args...).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	}

	inputPayload := validPayload()
	inputPayload.DestinationOfficeCode = ""
	payload, err := NewMySQLRepository(database).ValidateReferences(context.Background(), inputPayload, "40005CC", 7)
	if err != nil {
		t.Fatal(err)
	}
	if payload.DestinationOfficeCode != "40008CC" {
		t.Fatalf("resolved destination = %q, want shipment target office", payload.DestinationOfficeCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReferencesRejectsInactiveOrUnknownMaster(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Apakah_Aktif = 'Y'")).
		WithArgs("40005CC").WillReturnRows(sqlmock.NewRows([]string{"exists"}))
	if _, err := NewMySQLRepository(database).ValidateReferences(context.Background(), validPayload(), "40005CC", 7); !errors.Is(err, submission.ErrInvalidSubmission) {
		t.Fatalf("ValidateReferences() error = %v, want invalid submission", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindCustomerMatchesUsesExactLegacyIdentityFieldsAndCapsAmbiguity(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	query := matchCustomerBaseQuery + " AND Email = ? ORDER BY ID_Pelanggan ASC LIMIT 2"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("CUSTOMER INPUT", "0812345678", "customer@example.test").
		WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan", "Nama_Lengkap", "Email", "Nomor_HP"}).
			AddRow(42, "CUSTOMER INPUT", "customer@example.test", "0812345678"))
	profiles, err := NewMySQLRepository(database).FindCustomerMatches(context.Background(), validPayload())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ID != 42 || profiles[0].Email != "customer@example.test" {
		t.Fatalf("unexpected customer matches: %#v", profiles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetDraftRestoresConsentAndPayloadState(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.September, 30, 9, 0, 0, 0, time.UTC)
	consentAt := createdAt.Add(time.Minute)
	payloadJSON, err := json.Marshal(validPayload())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(getDraftQuery)).
		WithArgs("conversation-1").
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "status", "review_reason", "ticket_id", "customer_id", "intake_payload", "summary", "consent_at", "created_at"}).
			AddRow("conversation-1", "creating", nil, nil, 42, string(payloadJSON), "SUMMARY", consentAt, createdAt))

	draft, found, err := NewMySQLRepository(database).GetDraft(context.Background(), "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if !found || draft.Status != "creating" || draft.CustomerID != 42 || draft.Payload.CustomerName != "CUSTOMER INPUT" || !draft.ConsentAt.Equal(consentAt) {
		t.Fatalf("unexpected draft state: found=%v draft=%#v", found, draft)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInsertDraftIfAbsentClaimsConversationOnce(t *testing.T) {
	for _, test := range []struct {
		name         string
		rowsAffected int64
		wantClaimed  bool
	}{
		{name: "first submit", rowsAffected: 1, wantClaimed: true},
		{name: "replay", rowsAffected: 0, wantClaimed: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			payloadJSON, err := json.Marshal(validPayload())
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectExec(regexp.QuoteMeta(insertDraftQuery)).
				WithArgs("conversation-1", "creating", int64(42), string(payloadJSON), "SUMMARY", nil).
				WillReturnResult(sqlmock.NewResult(1, test.rowsAffected))
			draft := submission.Draft{ConversationID: "conversation-1", Status: "creating", CustomerID: 42, Payload: validPayload(), Summary: "SUMMARY"}
			claimed, err := NewMySQLRepository(database).InsertDraftIfAbsent(context.Background(), draft)
			if err != nil {
				t.Fatal(err)
			}
			if claimed != test.wantClaimed {
				t.Fatalf("claimed=%v, want %v", claimed, test.wantClaimed)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFindMatchingTicketIDsUsesExactPayloadAndConsentWindow(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	consentAt := time.Date(2026, time.September, 30, 9, 0, 0, 0, time.UTC)
	draft := submission.Draft{CustomerID: 42, Payload: validPayload(), Summary: "SUMMARY", ConsentAt: consentAt}
	mock.ExpectQuery(regexp.QuoteMeta(matchTicketQuery)).
		WithArgs(int64(42), "TEST-WAYBILL-001", "40005CC", "40006CC", int64(1), int64(7), "SUMMARY", "Complaint narrative", consentAt).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Pengaduan"}).AddRow("40005CC202609300003"))

	ticketIDs, err := NewMySQLRepository(database).FindMatchingTicketIDs(context.Background(), draft, "40005CC", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticketIDs) != 1 || ticketIDs[0] != "40005CC202609300003" {
		t.Fatalf("unexpected reconciliation IDs: %#v", ticketIDs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func validPayload() submission.Payload {
	return submission.Payload{
		CustomerName:          "CUSTOMER INPUT",
		CustomerEmail:         "customer@example.test",
		CustomerPhone:         "0812345678",
		CustomerAddress:       "ADDRESS INPUT",
		DestinationOfficeCode: "40006CC",
		ComplaintTypeCode:     1,
		ShippingServiceCode:   "REG",
		ShipmentNumber:        "TEST-WAYBILL-001",
		ShipmentOriginOffice:  "40007CC",
		ShipmentTargetOffice:  "40008CC",
		ComplaintDescription:  "Complaint narrative",
	}
}
