package submission

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/complaints/gateway/legacycch"
	"cch.local/backend/internal/modules/intake/session"
	"cch.local/backend/internal/platform/authentication"
)

const (
	complaintTableName = "cchentri"
	callCenterRoleID   = 3
	liveChatSourceID   = 7
	recoveryGrace      = time.Minute
)

var (
	ErrServiceNotConfigured = errors.New("guest intake submission service is not configured")
	ErrConsentRequired      = errors.New("explicit customer consent is required")
	ErrInvalidSubmission    = errors.New("invalid guest intake submission")
	ErrInvalidSession       = errors.New("invalid guest intake session")
	ErrCallCenterRequired   = errors.New("intake CCH identity must be a single Call Center account")
	ErrIntakeOfficeMismatch = errors.New("intake CCH account does not match configured origin office")
	ErrCustomerNotFound     = errors.New("customer profile not found or ambiguous; CS review required")
	ErrDraftState           = errors.New("guest intake draft is not in a submittable state")
	ErrSubmissionProcessing = errors.New("guest intake submission is still being reconciled")
	ErrCCHUnavailable       = errors.New("CCH ticket creation is unavailable; submission requires review")
)

type Payload struct {
	CustomerName          string `json:"Nama_Pelanggan"`
	CustomerEmail         string `json:"Email_Pelanggan,omitempty"`
	CustomerPhone         string `json:"Nomor_HP_Pelanggan"`
	CustomerAddress       string `json:"Alamat_Pelanggan"`
	DestinationOfficeCode string `json:"Kantor_Tujuan,omitempty"`
	ComplaintTypeCode     int64  `json:"Jenis_Pengaduan"`
	ShippingServiceCode   string `json:"Jenis_Kiriman"`
	ShipmentNumber        string `json:"Nomor_Kiriman"`
	ShipmentOriginOffice  string `json:"Kantor_Asal_Kiriman"`
	ShipmentTargetOffice  string `json:"Kantor_Tujuan_Kiriman"`
	ComplaintDescription  string `json:"Uraian_Pengaduan"`
}

type Request struct {
	Consent bool    `json:"consent"`
	Summary string  `json:"summary"`
	Payload Payload `json:"payload"`
}

func (request Request) Normalize() (Request, error) {
	if !request.Consent {
		return Request{}, ErrConsentRequired
	}
	request.Summary = strings.TrimSpace(request.Summary)
	if request.Summary == "" || utf8.RuneCountInString(request.Summary) > 100 {
		return Request{}, ErrInvalidSubmission
	}
	request.Payload.CustomerName = strings.TrimSpace(request.Payload.CustomerName)
	request.Payload.CustomerEmail = strings.TrimSpace(request.Payload.CustomerEmail)
	request.Payload.CustomerPhone = strings.TrimSpace(request.Payload.CustomerPhone)
	request.Payload.CustomerAddress = strings.TrimSpace(request.Payload.CustomerAddress)
	request.Payload.DestinationOfficeCode = strings.TrimSpace(request.Payload.DestinationOfficeCode)
	request.Payload.ShippingServiceCode = strings.TrimSpace(request.Payload.ShippingServiceCode)
	request.Payload.ShipmentNumber = strings.TrimSpace(request.Payload.ShipmentNumber)
	request.Payload.ShipmentOriginOffice = strings.TrimSpace(request.Payload.ShipmentOriginOffice)
	request.Payload.ShipmentTargetOffice = strings.TrimSpace(request.Payload.ShipmentTargetOffice)
	request.Payload.ComplaintDescription = strings.TrimSpace(request.Payload.ComplaintDescription)
	if request.Payload.CustomerName == "" || utf8.RuneCountInString(request.Payload.CustomerName) > 50 ||
		request.Payload.CustomerPhone == "" || utf8.RuneCountInString(request.Payload.CustomerPhone) > 100 ||
		request.Payload.CustomerAddress == "" || utf8.RuneCountInString(request.Payload.CustomerAddress) > 5000 ||
		request.Payload.ComplaintTypeCode < 1 || request.Payload.ComplaintTypeCode > 255 ||
		request.Payload.ShippingServiceCode == "" || len(request.Payload.ShippingServiceCode) > 5 ||
		request.Payload.ShipmentNumber == "" || len(request.Payload.ShipmentNumber) > 255 ||
		request.Payload.ShipmentOriginOffice == "" || len(request.Payload.ShipmentOriginOffice) > 7 ||
		request.Payload.ShipmentTargetOffice == "" || len(request.Payload.ShipmentTargetOffice) > 7 ||
		request.Payload.ComplaintDescription == "" || utf8.RuneCountInString(request.Payload.ComplaintDescription) > 5000 {
		return Request{}, ErrInvalidSubmission
	}
	if len(request.Payload.DestinationOfficeCode) > 7 {
		return Request{}, ErrInvalidSubmission
	}
	if request.Payload.CustomerEmail != "" {
		address, err := mail.ParseAddress(request.Payload.CustomerEmail)
		if err != nil || address.Address != request.Payload.CustomerEmail || utf8.RuneCountInString(request.Payload.CustomerEmail) > 100 {
			return Request{}, ErrInvalidSubmission
		}
	}
	return request, nil
}

type CustomerProfile struct {
	ID       int64
	FullName string
	Email    string
	Phone    string
}

type Draft struct {
	ConversationID string
	Status         string
	ReviewReason   string
	TicketID       string
	CustomerID     int64
	Payload        Payload
	Summary        string
	ConsentAt      time.Time
	CreatedAt      time.Time
}

type Result struct {
	Status   string
	TicketID string
	Reason   string
}

type Repository interface {
	GetDraft(context.Context, string) (Draft, bool, error)
	ValidateReferences(context.Context, Payload, string, int64) (Payload, error)
	FindCustomerMatches(context.Context, Payload) ([]CustomerProfile, error)
	InsertDraftIfAbsent(context.Context, Draft) (bool, error)
	MarkDraftCreated(context.Context, string, string) error
	MarkDraftPendingCS(context.Context, string, string) error
	FindMatchingTicketIDs(context.Context, Draft, string, int64) ([]string, error)
}

type ConversationTokenVerifier interface {
	Verify(string) (session.VerifiedConversation, error)
}

type CCHTokenVerifier interface {
	Verify(string) (authentication.VerifiedIdentity, error)
}

type CCHAccessAuthorizer interface {
	Authorize(context.Context, authentication.VerifiedIdentity, string, authorization.Action) (authorization.AccountAccess, error)
}

type TicketGateway interface {
	Login(context.Context, string, string) (string, error)
	CreateComplaint(context.Context, string, legacycch.ComplaintPayload) (legacycch.CreateComplaintResult, error)
}

type Settings struct {
	OriginOfficeCode string
	SourceID         int64
	ServiceUsername  string
	ServicePassword  string
}

type Service struct {
	conversationVerifier ConversationTokenVerifier
	cchVerifier          CCHTokenVerifier
	accessAuthorizer     CCHAccessAuthorizer
	repository           Repository
	ticketGateway        TicketGateway
	settings             Settings
	currentTime          func() time.Time
}

func NewService(conversationVerifier ConversationTokenVerifier, cchVerifier CCHTokenVerifier, accessAuthorizer CCHAccessAuthorizer, repository Repository, ticketGateway TicketGateway, settings Settings) (*Service, error) {
	if conversationVerifier == nil || cchVerifier == nil || accessAuthorizer == nil || repository == nil || ticketGateway == nil {
		return nil, ErrServiceNotConfigured
	}
	settings.OriginOfficeCode = strings.TrimSpace(settings.OriginOfficeCode)
	if settings.OriginOfficeCode == "" || len(settings.OriginOfficeCode) > 7 || strings.Contains(settings.OriginOfficeCode, ",") ||
		settings.SourceID < 1 || strings.TrimSpace(settings.ServiceUsername) == "" || strings.TrimSpace(settings.ServicePassword) == "" {
		return nil, ErrServiceNotConfigured
	}
	return &Service{
		conversationVerifier: conversationVerifier,
		cchVerifier:          cchVerifier,
		accessAuthorizer:     accessAuthorizer,
		repository:           repository,
		ticketGateway:        ticketGateway,
		settings:             settings,
		currentTime:          time.Now,
	}, nil
}

func (service *Service) Submit(ctx context.Context, conversationToken string, request Request) (Result, error) {
	if service == nil || service.repository == nil || service.ticketGateway == nil {
		return Result{}, ErrServiceNotConfigured
	}
	conversation, err := service.conversationVerifier.Verify(conversationToken)
	if err != nil {
		return Result{}, ErrInvalidSession
	}
	request, err = request.Normalize()
	if err != nil {
		return Result{}, err
	}
	priorDraft, exists, err := service.repository.GetDraft(ctx, conversation.ConversationID)
	if err != nil {
		return Result{}, err
	}
	if exists {
		return service.resolvePriorDraft(ctx, priorDraft)
	}
	payload, err := service.repository.ValidateReferences(ctx, request.Payload, service.settings.OriginOfficeCode, service.settings.SourceID)
	if err != nil {
		return Result{}, err
	}
	profiles, err := service.repository.FindCustomerMatches(ctx, payload)
	if err != nil {
		return Result{}, err
	}
	if len(profiles) != 1 {
		draft := Draft{
			ConversationID: conversation.ConversationID,
			Status:         "pending_cs",
			ReviewReason:   "profile_unmatched",
			Payload:        payload,
			Summary:        request.Summary,
		}
		inserted, err := service.repository.InsertDraftIfAbsent(ctx, draft)
		if err != nil {
			return Result{}, err
		}
		if !inserted {
			storedDraft, found, err := service.repository.GetDraft(ctx, conversation.ConversationID)
			if err != nil || !found {
				return Result{}, ErrSubmissionProcessing
			}
			return service.resolvePriorDraft(ctx, storedDraft)
		}
		return Result{Status: "pending_cs", Reason: "profile_unmatched"}, nil
	}

	serviceToken, err := service.ticketGateway.Login(ctx, service.settings.ServiceUsername, service.settings.ServicePassword)
	if err != nil {
		return Result{}, ErrCCHUnavailable
	}
	identity, err := service.cchVerifier.Verify(serviceToken)
	if err != nil {
		return Result{}, ErrCCHUnavailable
	}
	access, err := service.accessAuthorizer.Authorize(ctx, identity, complaintTableName, authorization.ActionAdd)
	if err != nil {
		return Result{}, ErrCCHUnavailable
	}
	if len(access.UserLevelIDs) != 1 || access.UserLevelIDs[0] != callCenterRoleID {
		return Result{}, ErrCallCenterRequired
	}
	if access.OfficeCode != service.settings.OriginOfficeCode {
		return Result{}, ErrIntakeOfficeMismatch
	}

	profile := profiles[0]
	ticketPayload := legacycch.ComplaintPayload{
		CustomerID:            profile.ID,
		CustomerName:          profile.FullName,
		CustomerEmail:         profile.Email,
		CustomerPhone:         profile.Phone,
		CustomerAddress:       payload.CustomerAddress,
		OriginOfficeCode:      service.settings.OriginOfficeCode,
		DestinationOfficeCode: payload.DestinationOfficeCode,
		ComplaintTypeCode:     payload.ComplaintTypeCode,
		ComplaintSourceCode:   service.settings.SourceID,
		ShippingServiceCode:   payload.ShippingServiceCode,
		ShipmentNumber:        payload.ShipmentNumber,
		ShipmentOriginOffice:  payload.ShipmentOriginOffice,
		ShipmentTargetOffice:  payload.ShipmentTargetOffice,
		ComplaintSummary:      request.Summary,
		ComplaintDescription:  payload.ComplaintDescription,
	}
	if err := ticketPayload.Validate(); err != nil {
		return Result{}, err
	}
	draft := Draft{
		ConversationID: conversation.ConversationID,
		Status:         "creating",
		CustomerID:     profile.ID,
		Payload:        payload,
		Summary:        request.Summary,
	}
	inserted, err := service.repository.InsertDraftIfAbsent(ctx, draft)
	if err != nil {
		return Result{}, err
	}
	if !inserted {
		storedDraft, found, err := service.repository.GetDraft(ctx, conversation.ConversationID)
		if err != nil || !found {
			return Result{}, ErrSubmissionProcessing
		}
		return service.resolvePriorDraft(ctx, storedDraft)
	}
	storedDraft, found, err := service.repository.GetDraft(ctx, conversation.ConversationID)
	if err != nil || !found {
		return Result{}, ErrSubmissionProcessing
	}
	created, err := service.ticketGateway.CreateComplaint(ctx, serviceToken, ticketPayload)
	if err == nil {
		if err := service.repository.MarkDraftCreated(ctx, conversation.ConversationID, created.TicketID); err != nil {
			return Result{}, ErrSubmissionProcessing
		}
		return Result{Status: "created", TicketID: created.TicketID}, nil
	}
	return service.reconcileOrReview(ctx, storedDraft)
}

func (service *Service) resolvePriorDraft(ctx context.Context, draft Draft) (Result, error) {
	switch draft.Status {
	case "created":
		return Result{Status: "created", TicketID: draft.TicketID}, nil
	case "pending_cs":
		return Result{Status: "pending_cs", Reason: draft.ReviewReason}, nil
	case "creating":
		if service.currentTime().Sub(draft.CreatedAt) < recoveryGrace {
			return Result{Status: "creating"}, nil
		}
		return service.reconcileOrReview(ctx, draft)
	default:
		return Result{}, ErrDraftState
	}
}

func (service *Service) reconcileOrReview(ctx context.Context, draft Draft) (Result, error) {
	tickets, err := service.repository.FindMatchingTicketIDs(ctx, draft, service.settings.OriginOfficeCode, service.settings.SourceID)
	if err != nil {
		return Result{}, ErrSubmissionProcessing
	}
	if len(tickets) == 1 {
		if err := service.repository.MarkDraftCreated(ctx, draft.ConversationID, tickets[0]); err != nil {
			return Result{}, ErrSubmissionProcessing
		}
		return Result{Status: "created", TicketID: tickets[0]}, nil
	}
	reason := "ticket_creation_failed"
	if len(tickets) > 1 {
		reason = "ticket_creation_ambiguous"
	}
	if err := service.repository.MarkDraftPendingCS(ctx, draft.ConversationID, reason); err != nil {
		return Result{}, ErrSubmissionProcessing
	}
	return Result{Status: "pending_cs", Reason: reason}, nil
}
