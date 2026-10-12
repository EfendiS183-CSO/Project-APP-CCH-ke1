package submission

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/complaints/gateway/legacycch"
	"cch.local/backend/internal/modules/intake/session"
	"cch.local/backend/internal/platform/authentication"
)

type conversationVerifierStub struct {
	conversation session.VerifiedConversation
	err          error
	calls        int
}

func (verifier *conversationVerifierStub) Verify(_ string) (session.VerifiedConversation, error) {
	verifier.calls++
	return verifier.conversation, verifier.err
}

type cchVerifierStub struct {
	identity authentication.VerifiedIdentity
	err      error
}

func (verifier *cchVerifierStub) Verify(_ string) (authentication.VerifiedIdentity, error) {
	return verifier.identity, verifier.err
}

type accessAuthorizerStub struct {
	access authorization.AccountAccess
	err    error
	table  string
	action authorization.Action
	calls  int
}

func (authorizer *accessAuthorizerStub) Authorize(_ context.Context, _ authentication.VerifiedIdentity, table string, action authorization.Action) (authorization.AccountAccess, error) {
	authorizer.calls++
	authorizer.table = table
	authorizer.action = action
	return authorizer.access, authorizer.err
}

type repositoryStub struct {
	draft       Draft
	draftExists bool
	profiles    []CustomerProfile
	prepared    Payload
	inserted    bool
	findTickets []string
	insertCalls int
	markCreated string
	markPending string
	err         error
}

func (repository *repositoryStub) GetDraft(_ context.Context, _ string) (Draft, bool, error) {
	return repository.draft, repository.draftExists, repository.err
}

func (repository *repositoryStub) ValidateReferences(_ context.Context, payload Payload, officeCode string, sourceID int64) (Payload, error) {
	if officeCode != "40005CC" || sourceID != liveChatSourceID {
		return Payload{}, ErrInvalidSubmission
	}
	if repository.prepared.DestinationOfficeCode != "" {
		payload.DestinationOfficeCode = repository.prepared.DestinationOfficeCode
	}
	return payload, repository.err
}

func (repository *repositoryStub) FindCustomerMatches(context.Context, Payload) ([]CustomerProfile, error) {
	return repository.profiles, repository.err
}

func (repository *repositoryStub) InsertDraftIfAbsent(_ context.Context, draft Draft) (bool, error) {
	repository.insertCalls++
	if !repository.inserted {
		return false, repository.err
	}
	repository.draft = draft
	repository.draftExists = true
	return true, repository.err
}

func (repository *repositoryStub) MarkDraftCreated(_ context.Context, _ string, ticketID string) error {
	repository.markCreated = ticketID
	repository.draft.Status = "created"
	repository.draft.TicketID = ticketID
	return repository.err
}

func (repository *repositoryStub) MarkDraftPendingCS(_ context.Context, _ string, reason string) error {
	repository.markPending = reason
	repository.draft.Status = "pending_cs"
	repository.draft.ReviewReason = reason
	return repository.err
}

func (repository *repositoryStub) FindMatchingTicketIDs(context.Context, Draft, string, int64) ([]string, error) {
	return repository.findTickets, repository.err
}

type ticketGatewayStub struct {
	loginToken   string
	loginErr     error
	createResult legacycch.CreateComplaintResult
	createErr    error
	loginCalls   int
	createCalls  int
	payload      legacycch.ComplaintPayload
}

func (gateway *ticketGatewayStub) Login(context.Context, string, string) (string, error) {
	gateway.loginCalls++
	return gateway.loginToken, gateway.loginErr
}

func (gateway *ticketGatewayStub) CreateComplaint(_ context.Context, _ string, payload legacycch.ComplaintPayload) (legacycch.CreateComplaintResult, error) {
	gateway.createCalls++
	gateway.payload = payload
	return gateway.createResult, gateway.createErr
}

func TestSubmitRequiresConsentBeforeDatabaseOrCCHWork(t *testing.T) {
	service, repository, gateway := newTestService(t)
	request := validRequest()
	request.Consent = false
	if _, err := service.Submit(context.Background(), "guest-token", request); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("Submit() error = %v, want consent required", err)
	}
	if repository.insertCalls != 0 || gateway.loginCalls != 0 || gateway.createCalls != 0 {
		t.Fatalf("consent rejection had side effects: repository=%d login=%d create=%d", repository.insertCalls, gateway.loginCalls, gateway.createCalls)
	}
}

func TestRequestNormalizeEnforcesLegacyTextFieldLimits(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Request)
	}{
		{name: "customer address", change: func(request *Request) { request.Payload.CustomerAddress = strings.Repeat("a", 5001) }},
		{name: "complaint narrative", change: func(request *Request) { request.Payload.ComplaintDescription = strings.Repeat("a", 5001) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := validRequest()
			test.change(&request)
			if _, err := request.Normalize(); !errors.Is(err, ErrInvalidSubmission) {
				t.Fatalf("Normalize() error = %v, want invalid submission", err)
			}
		})
	}
}

func TestSubmitStoresUnmatchedProfileForCSWithoutCreatingTicket(t *testing.T) {
	service, repository, gateway := newTestService(t)
	result, err := service.Submit(context.Background(), "guest-token", validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "pending_cs" || result.Reason != "profile_unmatched" || repository.draft.Status != "pending_cs" {
		t.Fatalf("unexpected unmatched result: result=%#v draft=%#v", result, repository.draft)
	}
	if repository.insertCalls != 1 || gateway.loginCalls != 0 || gateway.createCalls != 0 {
		t.Fatalf("unmatched profile side effects: repository=%d login=%d create=%d", repository.insertCalls, gateway.loginCalls, gateway.createCalls)
	}
}

func TestSubmitCreatesTicketForOneCanonicalProfile(t *testing.T) {
	service, repository, gateway := newTestService(t)
	repository.profiles = []CustomerProfile{{
		ID:       42,
		FullName: "CANONICAL NAME",
		Email:    "canonical@example.test",
		Phone:    "0812345678",
	}}
	gateway.createResult = legacycch.CreateComplaintResult{TicketID: "40005CC202609300001"}

	result, err := service.Submit(context.Background(), "guest-token", validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "created" || result.TicketID != gateway.createResult.TicketID || repository.markCreated != result.TicketID {
		t.Fatalf("unexpected create result: %#v", result)
	}
	if gateway.loginCalls != 1 || gateway.createCalls != 1 {
		t.Fatalf("CCH gateway calls login=%d create=%d, want 1 each", gateway.loginCalls, gateway.createCalls)
	}
	if gateway.payload.CustomerID != 42 || gateway.payload.CustomerName != "CANONICAL NAME" || gateway.payload.CustomerEmail != "canonical@example.test" || gateway.payload.OriginOfficeCode != "40005CC" || gateway.payload.ComplaintSourceCode != liveChatSourceID {
		t.Fatalf("ticket payload did not use canonical/server-owned values: %#v", gateway.payload)
	}
	if gateway.payload.DestinationOfficeCode != "40006CC" {
		t.Fatalf("resolved destination = %q, want 40006CC", gateway.payload.DestinationOfficeCode)
	}
}

func TestSubmitSendsAmbiguousProfilesToCSWithoutCCHCall(t *testing.T) {
	service, repository, gateway := newTestService(t)
	repository.profiles = []CustomerProfile{{ID: 42}, {ID: 43}}
	result, err := service.Submit(context.Background(), "guest-token", validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "pending_cs" || result.Reason != "profile_unmatched" || gateway.createCalls != 0 {
		t.Fatalf("ambiguous profiles were not held for CS: result=%#v gateway=%#v", result, gateway)
	}
}

func TestSubmitRequiresServiceAccountCallCenterRoleAndConfiguredOffice(t *testing.T) {
	for _, test := range []struct {
		name   string
		access authorization.AccountAccess
		want   error
	}{
		{
			name: "wrong role",
			access: authorization.AccountAccess{
				OfficeCode:   "40005CC",
				UserLevelIDs: []int64{1},
			},
			want: ErrCallCenterRequired,
		},
		{
			name: "wrong office",
			access: authorization.AccountAccess{
				OfficeCode:   "40006CC",
				UserLevelIDs: []int64{callCenterRoleID},
			},
			want: ErrIntakeOfficeMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := validRequest().Payload
			prepared := payload
			prepared.DestinationOfficeCode = "40006CC"
			repository := &repositoryStub{
				profiles: []CustomerProfile{{ID: 42, FullName: "CANONICAL NAME", Phone: "0812345678"}},
				prepared: prepared,
				inserted: true,
			}
			gateway := &ticketGatewayStub{loginToken: "valid-cch-token"}
			authorizer := &accessAuthorizerStub{access: test.access}
			service, err := NewService(
				&conversationVerifierStub{conversation: session.VerifiedConversation{ConversationID: "conversation-1"}},
				&cchVerifierStub{identity: authentication.VerifiedIdentity{Username: "intake-bot"}},
				authorizer,
				repository,
				gateway,
				Settings{OriginOfficeCode: "40005CC", SourceID: liveChatSourceID, ServiceUsername: "intake-bot", ServicePassword: "test-password"},
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Submit(context.Background(), "guest-token", validRequest()); !errors.Is(err, test.want) {
				t.Fatalf("Submit() error = %v, want %v", err, test.want)
			}
			if repository.insertCalls != 0 || gateway.createCalls != 0 {
				t.Fatalf("authorization rejection had write side effects: draft inserts=%d CCH adds=%d", repository.insertCalls, gateway.createCalls)
			}
		})
	}
}

func TestSubmitRetryReturnsCreatedDraftWithoutSecondCCHCall(t *testing.T) {
	service, repository, gateway := newTestService(t)
	repository.draftExists = true
	repository.draft = Draft{ConversationID: "conversation-1", Status: "created", TicketID: "TICKET-EXISTING"}
	result, err := service.Submit(context.Background(), "guest-token", validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "created" || result.TicketID != "TICKET-EXISTING" || gateway.loginCalls != 0 || gateway.createCalls != 0 {
		t.Fatalf("existing created draft was not idempotent: result=%#v gateway=%#v", result, gateway)
	}
}

func TestSubmitReconcilesCreatingDraftWithoutReissuingCCHAdd(t *testing.T) {
	service, repository, gateway := newTestService(t)
	repository.draftExists = true
	repository.draft = Draft{
		ConversationID: "conversation-1",
		Status:         "creating",
		CreatedAt:      time.Now().Add(-2 * recoveryGrace),
		CustomerID:     42,
		Payload:        validRequest().Payload,
		Summary:        validRequest().Summary,
		ConsentAt:      time.Now().Add(-recoveryGrace),
	}
	repository.findTickets = []string{"TICKET-RECONCILED"}
	result, err := service.Submit(context.Background(), "guest-token", validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "created" || result.TicketID != "TICKET-RECONCILED" || repository.markCreated != result.TicketID || gateway.createCalls != 0 {
		t.Fatalf("creating draft was not reconciled idempotently: result=%#v repository=%#v gateway=%#v", result, repository, gateway)
	}
}

func TestSubmitMapsCCHFailureToCSReviewWhenReconciliationFindsNoTicket(t *testing.T) {
	service, repository, gateway := newTestService(t)
	repository.profiles = []CustomerProfile{{ID: 42, FullName: "CANONICAL NAME", Phone: "0812345678"}}
	gateway.createErr = errors.New("upstream timeout")
	result, err := service.Submit(context.Background(), "guest-token", validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "pending_cs" || result.Reason != "ticket_creation_failed" || repository.markPending != result.Reason {
		t.Fatalf("failed CCH request was not retained for review: result=%#v", result)
	}
	if gateway.createCalls != 1 {
		t.Fatalf("CreateComplaint calls = %d, want 1", gateway.createCalls)
	}
}

func newTestService(t *testing.T) (*Service, *repositoryStub, *ticketGatewayStub) {
	t.Helper()
	preparedPayload := validRequest().Payload
	preparedPayload.DestinationOfficeCode = "40006CC"
	repository := &repositoryStub{inserted: true, prepared: preparedPayload}
	gateway := &ticketGatewayStub{loginToken: "valid-cch-token"}
	access := &accessAuthorizerStub{access: authorization.AccountAccess{
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionAdd),
	}}
	service, err := NewService(
		&conversationVerifierStub{conversation: session.VerifiedConversation{ConversationID: "conversation-1"}},
		&cchVerifierStub{identity: authentication.VerifiedIdentity{Username: "intake-bot"}},
		access,
		repository,
		gateway,
		Settings{OriginOfficeCode: "40005CC", SourceID: liveChatSourceID, ServiceUsername: "intake-bot", ServicePassword: "test-password"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return service, repository, gateway
}

func validRequest() Request {
	return Request{
		Consent: true,
		Summary: "Ringkasan pengaduan",
		Payload: Payload{
			CustomerName:         "CUSTOMER INPUT",
			CustomerEmail:        "customer@example.test",
			CustomerPhone:        "0812345678",
			CustomerAddress:      "ADDRESS INPUT",
			ComplaintTypeCode:    1,
			ShippingServiceCode:  "REG",
			ShipmentNumber:       "TEST-WAYBILL-001",
			ShipmentOriginOffice: "40007CC",
			ShipmentTargetOffice: "40008CC",
			ComplaintDescription: "Deskripsi pengaduan",
		},
	}
}
