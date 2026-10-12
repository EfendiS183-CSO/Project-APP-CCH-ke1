package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"cch.local/backend/internal/modules/intake/review"
	"cch.local/backend/internal/modules/intake/submission"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

type reviewerStub struct {
	drafts      []review.DraftSummary
	draft       review.PendingDraft
	err         error
	query       review.Query
	searchQuery string
	candidates  []review.CustomerCandidate
	id          int64
}

type ticketCreatorStub struct {
	result  submission.Result
	err     error
	calls   int
	draftID int64
	request review.CreateRequest
}

func (creator *ticketCreatorStub) CreateAfterVerification(_ context.Context, _ authentication.VerifiedIdentity, draftID int64, request review.CreateRequest) (submission.Result, error) {
	creator.calls++
	creator.draftID = draftID
	creator.request = request
	return creator.result, creator.err
}

func (reviewer *reviewerStub) ListPending(_ context.Context, _ authentication.VerifiedIdentity, query review.Query) ([]review.DraftSummary, error) {
	reviewer.query = query
	return reviewer.drafts, reviewer.err
}

func (reviewer *reviewerStub) GetPending(_ context.Context, _ authentication.VerifiedIdentity, id int64) (review.PendingDraft, error) {
	reviewer.id = id
	return reviewer.draft, reviewer.err
}

func (reviewer *reviewerStub) SearchCustomers(_ context.Context, _ authentication.VerifiedIdentity, query string) ([]review.CustomerCandidate, error) {
	reviewer.searchQuery = query
	return reviewer.candidates, reviewer.err
}

func TestReviewControllerListsPendingDraftsWithNoStoreAndJWT(t *testing.T) {
	reviewer := &reviewerStub{drafts: []review.DraftSummary{{ID: 12, Summary: "SUMMARY", ReviewReason: "profile_unmatched"}}}
	controller := New(reviewer, nil)
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(newTestJWTVerifier(t), controller))
	request := authenticatedRequest(t, http.MethodGet, "/api/v1/intake/review/drafts?limit=10&offset=5")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d cache-control=%q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	if reviewer.query != (review.Query{Limit: 10, Offset: 5}) {
		t.Fatalf("query = %#v", reviewer.query)
	}
}

func TestReviewControllerReturnsPendingDraftDetailAndHidesConversationTokenID(t *testing.T) {
	reviewer := &reviewerStub{draft: review.PendingDraft{
		ID: 12,
		Draft: submission.Draft{
			ConversationID: "private-conversation-id",
			Status:         "pending_cs",
			ReviewReason:   "profile_unmatched",
			Payload:        submission.Payload{CustomerName: "CUSTOMER"},
			Summary:        "SUMMARY",
			CreatedAt:      time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC),
		},
	}}
	controller := New(reviewer, nil)
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(newTestJWTVerifier(t), controller))
	response, err := app.Test(authenticatedRequest(t, http.MethodGet, "/api/v1/intake/review/drafts/12"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || reviewer.id != 12 {
		t.Fatalf("status=%d requested id=%d", response.StatusCode, reviewer.id)
	}
	var responseBody []byte
	responseBody, err = io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(responseBody), "private-conversation-id") {
		t.Fatal("conversation ownership identifier leaked in review response")
	}
}

func TestReviewControllerRequiresJWTAndValidDraftID(t *testing.T) {
	controller := New(&reviewerStub{}, nil)
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(newTestJWTVerifier(t), controller))
	unauthenticated, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/intake/review/drafts", nil))
	if err != nil {
		t.Fatal(err)
	}
	unauthenticated.Body.Close()
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.StatusCode)
	}
	invalidID, err := app.Test(authenticatedRequest(t, http.MethodGet, "/api/v1/intake/review/drafts/nope"))
	if err != nil {
		t.Fatal(err)
	}
	invalidID.Body.Close()
	if invalidID.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d", invalidID.StatusCode)
	}
}

func TestReviewControllerSearchesCustomersWithoutCachingPII(t *testing.T) {
	reviewer := &reviewerStub{candidates: []review.CustomerCandidate{{ID: 29, Name: "CUSTOMER", Phone: "0812345678"}}}
	controller := New(reviewer, nil)
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(newTestJWTVerifier(t), controller))
	response, err := app.Test(authenticatedRequest(t, http.MethodGet, "/api/v1/intake/review/customers/search?q=CUSTOMER"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" || reviewer.searchQuery != "CUSTOMER" {
		t.Fatalf("status=%d cache=%q query=%q", response.StatusCode, response.Header.Get("Cache-Control"), reviewer.searchQuery)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "0812345678") {
		t.Fatalf("customer candidate missing from response: %s", body)
	}
}

func TestReviewControllerCreatesTicketWithExplicitAttestation(t *testing.T) {
	creator := &ticketCreatorStub{result: submission.Result{Status: "created", TicketID: "TICKET-12"}}
	controller := New(&reviewerStub{}, creator)
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(newTestJWTVerifier(t), controller))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/intake/review/drafts/12/create-ticket", strings.NewReader(`{"customer_id":44,"identity_verified":true,"corrections":{"Kantor_Tujuan":"40007CC"}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Authorization", signedTestJWT(t))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated || response.Header.Get("Cache-Control") != "no-store" || creator.calls != 1 || creator.draftID != 12 || !creator.request.IdentityVerified {
		t.Fatalf("status=%d cache=%q creator=%#v", response.StatusCode, response.Header.Get("Cache-Control"), creator)
	}
}

func TestReviewControllerRejectsUnknownCreateFields(t *testing.T) {
	creator := &ticketCreatorStub{}
	controller := New(&reviewerStub{}, creator)
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(newTestJWTVerifier(t), controller))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/intake/review/drafts/12/create-ticket", strings.NewReader(`{"customer_id":44,"identity_verified":true,"conversation_id":"spoof"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Authorization", signedTestJWT(t))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || creator.calls != 0 {
		t.Fatalf("status=%d creator calls=%d", response.StatusCode, creator.calls)
	}
}

func newTestJWTVerifier(t *testing.T) *authentication.JWTVerifier {
	t.Helper()
	verifier, err := authentication.NewJWTVerifier(config.JWTSettings{Secret: "test-only-review-jwt-secret-at-least-32", Issuer: "cch-review-test"})
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func authenticatedRequest(t *testing.T, method string, target string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("X-Authorization", signedTestJWT(t))
	return request
}

func signedTestJWT(t *testing.T) string {
	t.Helper()
	const secret = "test-only-review-jwt-secret-at-least-32"
	claims := jwt.MapClaims{
		"iss":      "cch-review-test",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(time.Minute).Unix(),
		"security": map[string]any{"username": "reviewer.test", "userid": "40005CC", "userlevelid": "3"},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
