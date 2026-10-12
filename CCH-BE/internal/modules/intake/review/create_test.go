package review

import (
	"context"
	"errors"
	"testing"
	"time"

	"cch.local/backend/internal/modules/complaints/gateway/legacycch"
	"cch.local/backend/internal/modules/intake/submission"
	"cch.local/backend/internal/platform/authentication"
)

type cchVerifierStub struct {
	identity authentication.VerifiedIdentity
	err      error
}

func (verifier *cchVerifierStub) Verify(string) (authentication.VerifiedIdentity, error) {
	return verifier.identity, verifier.err
}

type draftReviewRepositoryStub struct {
	*repositoryStub
	profile        CustomerCandidate
	profileFound   bool
	profileCalls   int
	claim          bool
	claimCalls     int
	claimedID      int64
	claimedPayload submission.Payload
	claimedBy      string
	claimedAt      time.Time
}

func (repository *draftReviewRepositoryStub) GetCustomerByID(context.Context, int64) (CustomerCandidate, bool, error) {
	repository.profileCalls++
	return repository.profile, repository.profileFound, nil
}

func (repository *draftReviewRepositoryStub) ClaimPendingDraft(_ context.Context, _ int64, customerID int64, payload submission.Payload, reviewer string, reviewedAt time.Time) (bool, error) {
	repository.claimCalls++
	repository.claimedID = customerID
	repository.claimedPayload = payload
	repository.claimedBy = reviewer
	repository.claimedAt = reviewedAt
	return repository.claim, nil
}

type submissionStateStub struct {
	preparedPayload submission.Payload
	matchedTickets  []string
	markedCreated   string
	markedPending   string
	validated       bool
}

func (repository *submissionStateStub) ValidateReferences(_ context.Context, payload submission.Payload, origin string, sourceID int64) (submission.Payload, error) {
	repository.validated = true
	if origin != "40005CC" || sourceID != liveChatSourceID {
		return submission.Payload{}, submission.ErrInvalidSubmission
	}
	if repository.preparedPayload.DestinationOfficeCode != "" {
		payload.DestinationOfficeCode = repository.preparedPayload.DestinationOfficeCode
	}
	return payload, nil
}

func (repository *submissionStateStub) MarkDraftCreated(_ context.Context, _ string, ticketID string) error {
	repository.markedCreated = ticketID
	return nil
}

func (repository *submissionStateStub) MarkDraftPendingCS(_ context.Context, _ string, reason string) error {
	repository.markedPending = reason
	return nil
}

func (repository *submissionStateStub) FindMatchingTicketIDs(context.Context, submission.Draft, string, int64) ([]string, error) {
	return repository.matchedTickets, nil
}

type ticketGatewayStub struct {
	token       string
	loginErr    error
	createErr   error
	createCalls int
	payload     legacycch.ComplaintPayload
}

func (gateway *ticketGatewayStub) Login(context.Context, string, string) (string, error) {
	return gateway.token, gateway.loginErr
}

func (gateway *ticketGatewayStub) CreateComplaint(_ context.Context, _ string, payload legacycch.ComplaintPayload) (legacycch.CreateComplaintResult, error) {
	gateway.createCalls++
	gateway.payload = payload
	if gateway.createErr != nil {
		return legacycch.CreateComplaintResult{}, gateway.createErr
	}
	return legacycch.CreateComplaintResult{TicketID: "TICKET-CS-001"}, nil
}

func TestCreateAfterVerificationRequiresCSAttestationBeforeAnyWork(t *testing.T) {
	creator, repository, gateway, _ := newTicketCreator(t)
	request := validCreateRequest()
	request.IdentityVerified = false
	if _, err := creator.CreateAfterVerification(context.Background(), reviewerIdentity(), 12, request); !errors.Is(err, ErrIdentityVerificationRequired) {
		t.Fatalf("create error = %v, want attestation required", err)
	}
	if repository.profileCalls != 0 || repository.claimCalls != 0 || gateway.createCalls != 0 {
		t.Fatalf("unverified request had side effects: profile=%d claim=%d CCH=%d", repository.profileCalls, repository.claimCalls, gateway.createCalls)
	}
}

func TestCreateAfterVerificationUsesCanonicalProfileAndClaimsOnlyPendingDraft(t *testing.T) {
	creator, repository, gateway, state := newTicketCreator(t)
	result, err := creator.CreateAfterVerification(context.Background(), reviewerIdentity(), 12, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "created" || result.TicketID != "TICKET-CS-001" || repository.claimCalls != 1 || state.markedCreated != result.TicketID {
		t.Fatalf("unexpected create result=%#v repository=%#v state=%#v", result, repository, state)
	}
	if gateway.payload.CustomerID != 44 || gateway.payload.CustomerName != "CANONICAL NAME" ||
		gateway.payload.CustomerEmail != "canonical@example.test" || gateway.payload.CustomerPhone != "0812345678" ||
		gateway.payload.CustomerAddress != "CANONICAL ADDRESS" {
		t.Fatalf("ticket did not use canonical profile snapshot: %#v", gateway.payload)
	}
	if gateway.payload.DestinationOfficeCode != "40007CC" || gateway.payload.ShipmentOriginOffice != "40006CC" || gateway.payload.ShipmentTargetOffice != "40007CC" {
		t.Fatalf("allowed office corrections were not applied: %#v", gateway.payload)
	}
	if repository.claimedID != 44 || repository.claimedPayload.CustomerName != "CANONICAL NAME" || repository.claimedBy != "reviewer.test" || repository.claimedAt.IsZero() || !state.validated {
		t.Fatalf("claim/validation did not persist canonical payload: %#v %#v", repository, state)
	}
}

func TestCreateAfterVerificationRejectsUnclaimedDraftAndUntrustedCCHAccount(t *testing.T) {
	creator, repository, gateway, _ := newTicketCreator(t)
	repository.claim = false
	if _, err := creator.CreateAfterVerification(context.Background(), reviewerIdentity(), 12, validCreateRequest()); !errors.Is(err, submission.ErrDraftState) {
		t.Fatalf("claim race error = %v, want draft state conflict", err)
	}
	if gateway.createCalls != 0 {
		t.Fatal("CCH Add called when conditional pending claim failed")
	}

	creator, repository, gateway, _ = newTicketCreator(t)
	creator.cchVerifier = &cchVerifierStub{err: authentication.ErrInvalidToken}
	if _, err := creator.CreateAfterVerification(context.Background(), reviewerIdentity(), 12, validCreateRequest()); !errors.Is(err, ErrReviewCCHUnavailable) {
		t.Fatalf("invalid technical CCH token error = %v", err)
	}
	if repository.claimCalls != 0 || gateway.createCalls != 0 {
		t.Fatal("unverified technical CCH identity had draft/ticket side effects")
	}
}

func TestCreateAfterVerificationReconcilesFailedCCHCallWithoutReissuing(t *testing.T) {
	creator, _, gateway, state := newTicketCreator(t)
	gateway.createErr = errors.New("upstream timeout")
	state.matchedTickets = []string{"TICKET-RECONCILED"}
	result, err := creator.CreateAfterVerification(context.Background(), reviewerIdentity(), 12, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "created" || result.TicketID != "TICKET-RECONCILED" || state.markedCreated != result.TicketID || gateway.createCalls != 1 {
		t.Fatalf("failed upstream call was not reconciled: result=%#v state=%#v calls=%d", result, state, gateway.createCalls)
	}
}

func reviewerIdentity() authentication.VerifiedIdentity {
	return authentication.VerifiedIdentity{Username: "reviewer.test"}
}

func newTicketCreator(t *testing.T) (*TicketCreator, *draftReviewRepositoryStub, *ticketGatewayStub, *submissionStateStub) {
	t.Helper()
	baseRepository := &repositoryStub{
		found: true,
		draft: PendingDraft{ID: 12, Draft: submission.Draft{
			ConversationID: "conversation-12",
			Status:         "pending_cs",
			Summary:        "Complaint summary",
			ConsentAt:      time.Now().Add(-time.Minute),
			Payload:        validIntakePayload(),
		}},
	}
	repository := &draftReviewRepositoryStub{
		repositoryStub: baseRepository,
		profile: CustomerCandidate{
			ID: 44, Name: "CANONICAL NAME", Email: "canonical@example.test", Phone: "0812345678", Address: "CANONICAL ADDRESS",
		},
		profileFound: true,
		claim:        true,
	}
	authorizer := validAuthorizer()
	baseService, err := NewService(authorizer, repository, "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	state := &submissionStateStub{preparedPayload: submission.Payload{DestinationOfficeCode: "40007CC"}}
	gateway := &ticketGatewayStub{token: "valid-technical-token"}
	creator, err := NewTicketCreator(baseService, repository, state, &cchVerifierStub{identity: authentication.VerifiedIdentity{Username: "intake-bot"}}, gateway, TicketSettings{
		OriginOfficeCode: "40005CC", SourceID: liveChatSourceID, ServiceUsername: "intake-bot", ServicePassword: "test-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	return creator, repository, gateway, state
}

func validCreateRequest() CreateRequest {
	return CreateRequest{
		CustomerID:       44,
		IdentityVerified: true,
		Corrections: Corrections{
			DestinationOfficeCode: "40007CC",
			ShipmentOriginOffice:  "40006CC",
			ShipmentTargetOffice:  "40007CC",
		},
	}
}

func validIntakePayload() submission.Payload {
	return submission.Payload{
		CustomerName:          "GUEST NAME",
		CustomerEmail:         "guest@example.test",
		CustomerPhone:         "0899999999",
		CustomerAddress:       "GUEST ADDRESS",
		DestinationOfficeCode: "40008CC",
		ComplaintTypeCode:     1,
		ShippingServiceCode:   "REG",
		ShipmentNumber:        "WAYBILL-001",
		ShipmentOriginOffice:  "40005CC",
		ShipmentTargetOffice:  "40005CC",
		ComplaintDescription:  "Complaint detail",
	}
}
