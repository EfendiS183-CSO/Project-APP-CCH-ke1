package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/intake/review"
	"cch.local/backend/internal/modules/intake/submission"
)

func TestListPendingReturnsOnlyQueueProjectionAndHandlesNulls(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(listPendingQuery)).WithArgs(25, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "summary", "review_reason", "created_at"}).
			AddRow(12, nil, nil, createdAt))
	drafts, err := NewMySQLRepository(database).ListPending(context.Background(), reviewQuery(25, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 || drafts[0].ID != 12 || drafts[0].Summary != "" || drafts[0].ReviewReason != "" || !drafts[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected pending drafts: %#v", drafts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetPendingDecodesConsentedPayloadAndNullableFields(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC)
	consentAt := createdAt.Add(time.Minute)
	payload, err := json.Marshal(claimedDraftPayload{
		Payload:  submission.Payload{CustomerName: "CUSTOMER", CustomerPhone: "0812345678"},
		CSReview: reviewAttestation{Reviewer: "reviewer.test", IdentityVerified: true, ReviewedAt: consentAt},
	})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(getPendingQuery)).WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "status", "review_reason", "ticket_id", "customer_id", "intake_payload", "summary", "consent_at", "created_at"}).
			AddRow(12, "conversation-12", "pending_cs", "profile_unmatched", nil, nil, string(payload), "SUMMARY", consentAt, createdAt))
	draft, found, err := NewMySQLRepository(database).GetPending(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if !found || draft.ID != 12 || draft.Draft.Status != "pending_cs" || draft.Draft.Payload.CustomerName != "CUSTOMER" ||
		draft.Draft.ReviewReason != "profile_unmatched" || !draft.Draft.ConsentAt.Equal(consentAt) {
		t.Fatalf("unexpected pending draft: found=%v draft=%#v", found, draft)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetPendingHidesMissingOrAlreadyProcessedDraft(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getPendingQuery)).WithArgs(int64(13)).WillReturnError(sql.ErrNoRows)
	_, found, err := NewMySQLRepository(database).GetPending(context.Background(), 13)
	if err != nil || found {
		t.Fatalf("GetPending() = found %v, error %v; want not found without error", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchCustomersEscapesWildcardsAndHandlesNullableProfileData(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(searchCustomersQuery)).
		WithArgs("A%_\\B", "%A\\%\\_\\\\B%", "%A\\%\\_\\\\B%", "%A\\%\\_\\\\B%").
		WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan", "Nama_Lengkap", "Email", "Nomor_HP", "Alamat"}).
			AddRow(29, "CUSTOMER", nil, nil, nil))
	candidates, err := NewMySQLRepository(database).SearchCustomers(context.Background(), "A%_\\B")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != 29 || candidates[0].Email != "" || candidates[0].Phone != "" || candidates[0].Address != "" {
		t.Fatalf("unexpected candidate: %#v", candidates)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetCustomerByIDLoadsCanonicalProfileAndHandlesNulls(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getCustomerByIDQuery)).WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Pelanggan", "Nama_Lengkap", "Email", "Nomor_HP", "Alamat"}).
			AddRow(44, "CANONICAL NAME", nil, "0812345678", "CANONICAL ADDRESS"))
	profile, found, err := NewMySQLRepository(database).GetCustomerByID(context.Background(), 44)
	if err != nil {
		t.Fatal(err)
	}
	if !found || profile.ID != 44 || profile.Name != "CANONICAL NAME" || profile.Email != "" || profile.Address != "CANONICAL ADDRESS" {
		t.Fatalf("unexpected canonical profile: found=%v profile=%#v", found, profile)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimPendingDraftPersistsReviewedPayloadOnlyOnce(t *testing.T) {
	for _, test := range []struct {
		name string
		rows int64
		want bool
	}{{name: "claim", rows: 1, want: true}, {name: "already processed", rows: 0, want: false}} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			payload := submission.Payload{CustomerName: "CANONICAL NAME", DestinationOfficeCode: "40006CC"}
			reviewedAt := time.Date(2026, time.October, 1, 10, 30, 0, 0, time.UTC)
			payloadJSON, err := json.Marshal(claimedDraftPayload{
				Payload:  payload,
				CSReview: reviewAttestation{Reviewer: "reviewer.test", IdentityVerified: true, ReviewedAt: reviewedAt},
			})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectExec(regexp.QuoteMeta(claimPendingDraftQuery)).
				WithArgs(int64(44), string(payloadJSON), int64(12)).
				WillReturnResult(sqlmock.NewResult(0, test.rows))
			claimed, err := NewMySQLRepository(database).ClaimPendingDraft(context.Background(), 12, 44, payload, "reviewer.test", reviewedAt)
			if err != nil || claimed != test.want {
				t.Fatalf("ClaimPendingDraft() = %v, %v; want %v", claimed, err, test.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func reviewQuery(limit int, offset int) review.Query {
	return review.Query{Limit: limit, Offset: offset}
}
