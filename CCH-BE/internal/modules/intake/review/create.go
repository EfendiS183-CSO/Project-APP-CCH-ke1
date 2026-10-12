package review

import (
	"context"
	"errors"
	"strings"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/complaints/gateway/legacycch"
	"cch.local/backend/internal/modules/intake/submission"
	"cch.local/backend/internal/platform/authentication"
)

const liveChatSourceID = 7

var (
	ErrIdentityVerificationRequired = errors.New("CS identity verification attestation is required")
	ErrInvalidReviewRequest         = errors.New("invalid CS intake review request")
	ErrReviewProcessing             = errors.New("CS intake review is being reconciled")
	ErrReviewCCHUnavailable         = errors.New("CCH ticket creation requires review")
)

type Corrections struct {
	DestinationOfficeCode string `json:"Kantor_Tujuan,omitempty"`
	ShipmentOriginOffice  string `json:"Kantor_Asal_Kiriman,omitempty"`
	ShipmentTargetOffice  string `json:"Kantor_Tujuan_Kiriman,omitempty"`
}

type CreateRequest struct {
	CustomerID       int64       `json:"customer_id"`
	IdentityVerified bool        `json:"identity_verified"`
	Corrections      Corrections `json:"corrections"`
}

type TicketDraftRepository interface {
	Repository
	GetCustomerByID(context.Context, int64) (CustomerCandidate, bool, error)
	ClaimPendingDraft(context.Context, int64, int64, submission.Payload, string, time.Time) (bool, error)
}

type SubmissionStateRepository interface {
	ValidateReferences(context.Context, submission.Payload, string, int64) (submission.Payload, error)
	MarkDraftCreated(context.Context, string, string) error
	MarkDraftPendingCS(context.Context, string, string) error
	FindMatchingTicketIDs(context.Context, submission.Draft, string, int64) ([]string, error)
}

type TicketCreator struct {
	reviewService   *Service
	draftRepository TicketDraftRepository
	stateRepository SubmissionStateRepository
	cchVerifier     submission.CCHTokenVerifier
	ticketGateway   submission.TicketGateway
	settings        TicketSettings
}

type TicketSettings struct {
	OriginOfficeCode string
	SourceID         int64
	ServiceUsername  string
	ServicePassword  string
}

func NewTicketCreator(reviewService *Service, draftRepository TicketDraftRepository, stateRepository SubmissionStateRepository, cchVerifier submission.CCHTokenVerifier, ticketGateway submission.TicketGateway, settings TicketSettings) (*TicketCreator, error) {
	if reviewService == nil || draftRepository == nil || stateRepository == nil || cchVerifier == nil || ticketGateway == nil {
		return nil, ErrServiceNotConfigured
	}
	settings.OriginOfficeCode = strings.TrimSpace(settings.OriginOfficeCode)
	if settings.OriginOfficeCode == "" || settings.OriginOfficeCode != reviewService.intakeOffice ||
		settings.SourceID != liveChatSourceID || strings.TrimSpace(settings.ServiceUsername) == "" || strings.TrimSpace(settings.ServicePassword) == "" {
		return nil, ErrServiceNotConfigured
	}
	return &TicketCreator{
		reviewService:   reviewService,
		draftRepository: draftRepository,
		stateRepository: stateRepository,
		cchVerifier:     cchVerifier,
		ticketGateway:   ticketGateway,
		settings:        settings,
	}, nil
}

func (creator *TicketCreator) CreateAfterVerification(ctx context.Context, identity authentication.VerifiedIdentity, draftID int64, request CreateRequest) (submission.Result, error) {
	if creator == nil || creator.reviewService == nil {
		return submission.Result{}, ErrServiceNotConfigured
	}
	if err := creator.reviewService.authorizeReviewer(ctx, identity); err != nil {
		return submission.Result{}, err
	}
	if draftID <= 0 || request.CustomerID <= 0 {
		return submission.Result{}, ErrInvalidReviewRequest
	}
	if !request.IdentityVerified {
		return submission.Result{}, ErrIdentityVerificationRequired
	}
	draft, found, err := creator.draftRepository.GetPending(ctx, draftID)
	if err != nil {
		return submission.Result{}, err
	}
	if !found {
		return submission.Result{}, ErrDraftNotFound
	}
	if draft.Draft.ConsentAt.IsZero() {
		return submission.Result{}, ErrInvalidReviewRequest
	}
	profile, found, err := creator.draftRepository.GetCustomerByID(ctx, request.CustomerID)
	if err != nil {
		return submission.Result{}, err
	}
	if !found {
		return submission.Result{}, submission.ErrCustomerNotFound
	}
	payload := draft.Draft.Payload
	payload.CustomerName = profile.Name
	payload.CustomerEmail = profile.Email
	payload.CustomerPhone = profile.Phone
	payload.CustomerAddress = profile.Address
	applyCorrection(&payload.DestinationOfficeCode, request.Corrections.DestinationOfficeCode)
	applyCorrection(&payload.ShipmentOriginOffice, request.Corrections.ShipmentOriginOffice)
	applyCorrection(&payload.ShipmentTargetOffice, request.Corrections.ShipmentTargetOffice)
	normalized, err := (submission.Request{Consent: true, Summary: draft.Draft.Summary, Payload: payload}).Normalize()
	if err != nil {
		return submission.Result{}, ErrInvalidReviewRequest
	}
	preparedPayload, err := creator.stateRepository.ValidateReferences(ctx, normalized.Payload, creator.settings.OriginOfficeCode, creator.settings.SourceID)
	if err != nil {
		return submission.Result{}, err
	}
	serviceToken, err := creator.ticketGateway.Login(ctx, creator.settings.ServiceUsername, creator.settings.ServicePassword)
	if err != nil {
		return submission.Result{}, ErrReviewCCHUnavailable
	}
	serviceIdentity, err := creator.cchVerifier.Verify(serviceToken)
	if err != nil {
		return submission.Result{}, ErrReviewCCHUnavailable
	}
	serviceAccess, err := creator.reviewService.authorizer.Authorize(ctx, serviceIdentity, complaintTable, authorization.ActionAdd)
	if err != nil || !isIntakeCallCenter(serviceAccess, creator.settings.OriginOfficeCode) {
		return submission.Result{}, ErrReviewCCHUnavailable
	}
	ticketPayload := legacycch.ComplaintPayload{
		CustomerID:            profile.ID,
		CustomerName:          profile.Name,
		CustomerEmail:         profile.Email,
		CustomerPhone:         profile.Phone,
		CustomerAddress:       profile.Address,
		OriginOfficeCode:      creator.settings.OriginOfficeCode,
		DestinationOfficeCode: preparedPayload.DestinationOfficeCode,
		ComplaintTypeCode:     preparedPayload.ComplaintTypeCode,
		ComplaintSourceCode:   creator.settings.SourceID,
		ShippingServiceCode:   preparedPayload.ShippingServiceCode,
		ShipmentNumber:        preparedPayload.ShipmentNumber,
		ShipmentOriginOffice:  preparedPayload.ShipmentOriginOffice,
		ShipmentTargetOffice:  preparedPayload.ShipmentTargetOffice,
		ComplaintSummary:      normalized.Summary,
		ComplaintDescription:  preparedPayload.ComplaintDescription,
	}
	if err := ticketPayload.Validate(); err != nil {
		return submission.Result{}, ErrInvalidReviewRequest
	}
	claimed, err := creator.draftRepository.ClaimPendingDraft(ctx, draftID, profile.ID, preparedPayload, identity.Username, time.Now().UTC())
	if err != nil {
		return submission.Result{}, err
	}
	if !claimed {
		return submission.Result{}, submission.ErrDraftState
	}
	draft.Draft.Status = "creating"
	draft.Draft.CustomerID = profile.ID
	draft.Draft.Payload = preparedPayload
	draft.Draft.Summary = normalized.Summary
	created, createErr := creator.ticketGateway.CreateComplaint(ctx, serviceToken, ticketPayload)
	if createErr == nil {
		if err := creator.stateRepository.MarkDraftCreated(ctx, draft.Draft.ConversationID, created.TicketID); err != nil {
			return submission.Result{}, ErrReviewProcessing
		}
		return submission.Result{Status: "created", TicketID: created.TicketID}, nil
	}
	return creator.reconcileOrReview(ctx, draft.Draft)
}

func (creator *TicketCreator) reconcileOrReview(ctx context.Context, draft submission.Draft) (submission.Result, error) {
	tickets, err := creator.stateRepository.FindMatchingTicketIDs(ctx, draft, creator.settings.OriginOfficeCode, creator.settings.SourceID)
	if err != nil {
		return submission.Result{}, ErrReviewProcessing
	}
	if len(tickets) == 1 {
		if err := creator.stateRepository.MarkDraftCreated(ctx, draft.ConversationID, tickets[0]); err != nil {
			return submission.Result{}, ErrReviewProcessing
		}
		return submission.Result{Status: "created", TicketID: tickets[0]}, nil
	}
	reason := "ticket_creation_failed"
	if len(tickets) > 1 {
		reason = "ticket_creation_ambiguous"
	}
	if err := creator.stateRepository.MarkDraftPendingCS(ctx, draft.ConversationID, reason); err != nil {
		return submission.Result{}, ErrReviewProcessing
	}
	return submission.Result{Status: "pending_cs", Reason: reason}, nil
}

func applyCorrection(field *string, correction string) {
	if normalized := strings.TrimSpace(correction); normalized != "" {
		*field = normalized
	}
}
