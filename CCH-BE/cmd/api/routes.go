package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/announcements"
	announcementcontroller "cch.local/backend/internal/modules/announcements/controller"
	announcementrepository "cch.local/backend/internal/modules/announcements/repository"
	appsettings "cch.local/backend/internal/modules/appsettings"
	appsettingscontroller "cch.local/backend/internal/modules/appsettings/controller"
	appsettingsrepository "cch.local/backend/internal/modules/appsettings/repository"
	"cch.local/backend/internal/modules/authorization"
	authorizationrepository "cch.local/backend/internal/modules/authorization/repository"
	"cch.local/backend/internal/modules/breadcrumbs"
	breadcrumbcontroller "cch.local/backend/internal/modules/breadcrumbs/controller"
	breadcrumbrepository "cch.local/backend/internal/modules/breadcrumbs/repository"
	"cch.local/backend/internal/modules/complaints"
	complaintcontroller "cch.local/backend/internal/modules/complaints/controller"
	"cch.local/backend/internal/modules/complaints/gateway/legacycch"
	complaintrepository "cch.local/backend/internal/modules/complaints/repository"
	"cch.local/backend/internal/modules/csofficers"
	csofficercontroller "cch.local/backend/internal/modules/csofficers/controller"
	csofficerrepository "cch.local/backend/internal/modules/csofficers/repository"
	"cch.local/backend/internal/modules/customers"
	customercontroller "cch.local/backend/internal/modules/customers/controller"
	customerrepository "cch.local/backend/internal/modules/customers/repository"
	"cch.local/backend/internal/modules/documents"
	documentcontroller "cch.local/backend/internal/modules/documents/controller"
	documentrepository "cch.local/backend/internal/modules/documents/repository"
	documentstorage "cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/modules/followups"
	followupcontroller "cch.local/backend/internal/modules/followups/controller"
	followuprepository "cch.local/backend/internal/modules/followups/repository"
	"cch.local/backend/internal/modules/help"
	helparticlecontroller "cch.local/backend/internal/modules/help/controller"
	helprepository "cch.local/backend/internal/modules/help/repository"
	"cch.local/backend/internal/modules/intake/review"
	intakereviewcontroller "cch.local/backend/internal/modules/intake/review/controller"
	intakereviewrepository "cch.local/backend/internal/modules/intake/review/repository"
	"cch.local/backend/internal/modules/intake/session"
	intakesessioncontroller "cch.local/backend/internal/modules/intake/session/controller"
	"cch.local/backend/internal/modules/intake/submission"
	intakesubmissioncontroller "cch.local/backend/internal/modules/intake/submission/controller"
	intakerepository "cch.local/backend/internal/modules/intake/submission/repository"
	"cch.local/backend/internal/modules/masters/complainttypes"
	complainttypecontroller "cch.local/backend/internal/modules/masters/complainttypes/controller"
	complainttyperepository "cch.local/backend/internal/modules/masters/complainttypes/repository"
	"cch.local/backend/internal/modules/masters/documenttypes"
	documenttypecontroller "cch.local/backend/internal/modules/masters/documenttypes/controller"
	documenttyperepository "cch.local/backend/internal/modules/masters/documenttypes/repository"
	"cch.local/backend/internal/modules/masters/followupstatuses"
	followupstatuscontroller "cch.local/backend/internal/modules/masters/followupstatuses/controller"
	followupstatusrepository "cch.local/backend/internal/modules/masters/followupstatuses/repository"
	"cch.local/backend/internal/modules/masters/handlingstatuses"
	handlingstatuscontroller "cch.local/backend/internal/modules/masters/handlingstatuses/controller"
	handlingstatusrepository "cch.local/backend/internal/modules/masters/handlingstatuses/repository"
	"cch.local/backend/internal/modules/masters/helpcategories"
	helpcategorycontroller "cch.local/backend/internal/modules/masters/helpcategories/controller"
	helpcategoryrepository "cch.local/backend/internal/modules/masters/helpcategories/repository"
	"cch.local/backend/internal/modules/masters/holidaycalendar"
	holidaycalendarcontroller "cch.local/backend/internal/modules/masters/holidaycalendar/controller"
	holidaycalendarrepository "cch.local/backend/internal/modules/masters/holidaycalendar/repository"
	languagemaster "cch.local/backend/internal/modules/masters/languages"
	languagemastercontroller "cch.local/backend/internal/modules/masters/languages/controller"
	languagemasterrepository "cch.local/backend/internal/modules/masters/languages/repository"
	"cch.local/backend/internal/modules/masters/offices"
	officecontroller "cch.local/backend/internal/modules/masters/offices/controller"
	officerepository "cch.local/backend/internal/modules/masters/offices/repository"
	"cch.local/backend/internal/modules/masters/shippingservices"
	shippingservicecontroller "cch.local/backend/internal/modules/masters/shippingservices/controller"
	shippingservicerepository "cch.local/backend/internal/modules/masters/shippingservices/repository"
	"cch.local/backend/internal/modules/masters/sources"
	sourcecontroller "cch.local/backend/internal/modules/masters/sources/controller"
	sourcerepository "cch.local/backend/internal/modules/masters/sources/repository"
	timezonemaster "cch.local/backend/internal/modules/masters/timezones"
	timezonemastercontroller "cch.local/backend/internal/modules/masters/timezones/controller"
	timezonemasterrepository "cch.local/backend/internal/modules/masters/timezones/repository"
	"cch.local/backend/internal/modules/regionals"
	regionalcontroller "cch.local/backend/internal/modules/regionals/controller"
	regionalrepository "cch.local/backend/internal/modules/regionals/repository"
	"cch.local/backend/internal/modules/userlevels"
	userlevelcontroller "cch.local/backend/internal/modules/userlevels/controller"
	userlevelrepository "cch.local/backend/internal/modules/userlevels/repository"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

var errComplaintRouteRequiresDatabase = errors.New("Call Center complaint API requires configured MySQL")
var errCustomerSearchRouteRequiresDatabase = errors.New("Call Center customer search API requires configured MySQL")
var errCustomerProfileRouteRequiresDatabase = errors.New("customer profile API requires configured MySQL")
var errComplaintTypeMasterRouteRequiresDatabase = errors.New("complaint type master API requires configured MySQL")
var errComplaintSourceMasterRouteRequiresDatabase = errors.New("complaint source master API requires configured MySQL")
var errShippingServiceMasterRouteRequiresDatabase = errors.New("shipping service master API requires configured MySQL")
var errDocumentTypeMasterRouteRequiresDatabase = errors.New("document type master API requires configured MySQL")
var errHandlingStatusMasterRouteRequiresDatabase = errors.New("handling status master API requires configured MySQL")
var errOfficeMasterRouteRequiresDatabase = errors.New("office master API requires configured MySQL")
var errFollowUpStatusMasterRouteRequiresDatabase = errors.New("follow-up status master API requires configured MySQL")
var errHelpCategoryMasterRouteRequiresDatabase = errors.New("help category master API requires configured MySQL")
var errHolidayCalendarMasterRouteRequiresDatabase = errors.New("holiday calendar master API requires configured MySQL")
var errTimezoneMasterRouteRequiresDatabase = errors.New("timezone master API requires configured MySQL")
var errLanguagesMasterRouteRequiresDatabase = errors.New("languages master API requires configured MySQL")
var errAppSettingsRouteRequiresDatabase = errors.New("app settings API requires configured MySQL")
var errRegionalAdminRouteRequiresDatabase = errors.New("Regional admin API requires configured MySQL")
var errCSOfficerStatusRouteRequiresDatabase = errors.New("CS Officer status API requires configured MySQL")
var errUserLevelCatalogRouteRequiresDatabase = errors.New("user-level catalog API requires configured MySQL")
var errBreadcrumbLinksRouteRequiresDatabase = errors.New("breadcrumb links API requires configured MySQL")
var errHelpArticleRouteRequiresDatabase = errors.New("help article API requires configured MySQL")
var errAnnouncementRouteRequiresDatabase = errors.New("announcement API requires configured MySQL")
var errGuestIntakeSessionRequiresDatabase = errors.New("guest intake session API requires configured MySQL")
var errGuestIntakeSessionKeyMustBeSeparate = errors.New("guest intake session signing key must differ from CCH staff JWT key")
var errGuestIntakeSubmissionRequiresDatabase = errors.New("guest intake submission API requires configured MySQL")
var errGuestIntakeSubmissionRequiresSession = errors.New("guest intake submission API requires enabled session issuance configuration")
var errGuestIntakeSubmissionKeyMustBeSeparate = errors.New("guest intake signing key must differ from CCH staff JWT key")
var errGuestIntakeReviewRequiresDatabase = errors.New("guest intake review API requires configured MySQL")
var errStatusWriteRequiresLegacyFence = errors.New("Call Center status mutation requires the legacy status Add path to be fenced")
var errStatusWriteRequiresDatabase = errors.New("Call Center status mutation requires configured MySQL")
var errFollowUpPlanRequiresDatabase = errors.New("Call Center follow-up plan API requires configured MySQL")
var errFollowUpRealizationRequiresDatabase = errors.New("Call Center follow-up realization API requires configured MySQL")
var errFollowUpCancelRequiresDatabase = errors.New("Call Center follow-up cancellation API requires configured MySQL")
var errFollowUpEditRequiresDatabase = errors.New("Call Center follow-up edit API requires configured MySQL")
var errDocumentMetadataRequiresDatabase = errors.New("Call Center document metadata API requires configured MySQL")
var errDocumentUploadRequiresDatabase = errors.New("Call Center document upload API requires configured MySQL")
var errDocumentDownloadRequiresDatabase = errors.New("Call Center document download API requires configured MySQL")
var errDocumentPreviewRequiresDatabase = errors.New("Call Center document preview API requires configured MySQL")
var errDocumentDeleteRequiresDatabase = errors.New("Call Center document delete API requires configured MySQL")

func callCenterComplaintRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterComplaintReadAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errComplaintRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}

	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	complaintRepository := complaintrepository.NewMySQLComplaintRepository(database)
	complaintService := complaints.NewService(accessService, complaintRepository)
	complaintController := complaintcontroller.New(complaintService)
	followUpRepository := followuprepository.NewMySQLRepository(database)
	followUpService := followups.NewService(accessService, followUpRepository)
	followUpController := followupcontroller.New(followUpService)
	lookupService := complaints.NewCallCenterLookupService(accessService, complaintRepository)
	lookupController := complaintcontroller.NewLookupController(lookupService)
	// Order matters: the lookup controller owns the static path
	// /complaints/call-center/lookups. It must be registered before the
	// complaint controller, whose /complaints/call-center/:id pattern would
	// otherwise capture "lookups" as an ID and respond 404.
	routeModules := httpapi.VersionedRouteModuleGroup{lookupController, followUpController, complaintController}
	return httpapi.AuthenticatedRouteModule(verifier, routeModules), nil
}

func callCenterStatusWriteRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterStatusWriteAPIEnabled {
		return nil, nil
	}
	if !settings.LegacyStatusWritesFenced {
		return nil, errStatusWriteRequiresLegacyFence
	}
	if database == nil {
		return nil, errStatusWriteRequiresDatabase
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := complaintrepository.VerifyStatusTransitionSchema(ctx, database); err != nil {
		return nil, err
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	statusRepository := complaintrepository.NewMySQLStatusTransitionRepository(database)
	statusService := complaints.NewStatusTransitionService(accessService, statusRepository)
	statusController := complaintcontroller.NewStatusTransitionController(statusService)
	return httpapi.AuthenticatedRouteModule(verifier, statusController), nil
}

func callCenterFollowUpPlanRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterFollowUpPlanAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errFollowUpPlanRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	followUpRepository := followuprepository.NewMySQLRepository(database)
	planService := followups.NewCallCenterPlanService(accessService, followUpRepository)
	planController := followupcontroller.NewPlanController(planService)
	return httpapi.AuthenticatedRouteModule(verifier, planController), nil
}

func callCenterFollowUpRealizationRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterFollowUpRealizeAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errFollowUpRealizationRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	followUpRepository := followuprepository.NewMySQLRepository(database)
	realizationService := followups.NewCallCenterRealizationService(accessService, followUpRepository)
	realizationController := followupcontroller.NewRealizationController(realizationService)
	return httpapi.AuthenticatedRouteModule(verifier, realizationController), nil
}

func callCenterFollowUpCancelRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterFollowUpCancelAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errFollowUpCancelRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	followUpRepository := followuprepository.NewMySQLRepository(database)
	cancelService := followups.NewCallCenterPlanCancellationService(accessService, followUpRepository)
	cancelController := followupcontroller.NewCancelController(cancelService)
	return httpapi.AuthenticatedRouteModule(verifier, cancelController), nil
}

func callCenterFollowUpEditRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterFollowUpEditAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errFollowUpEditRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	followUpRepository := followuprepository.NewMySQLRepository(database)
	editService := followups.NewCallCenterPlanEditService(accessService, followUpRepository)
	editController := followupcontroller.NewEditController(editService)
	return httpapi.AuthenticatedRouteModule(verifier, editController), nil
}

func callCenterDocumentMetadataRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterDocumentMetadataAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errDocumentMetadataRequiresDatabase
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := documentrepository.VerifyUploadSchema(ctx, database); err != nil {
		return nil, err
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	documentRepository := documentrepository.NewMySQLRepository(database)
	documentService := documents.NewService(accessService, documentRepository)
	documentController := documentcontroller.New(documentService)
	return httpapi.AuthenticatedRouteModule(verifier, documentController), nil
}

func callCenterDocumentUploadRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterDocumentUploadAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errDocumentUploadRequiresDatabase
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := documentrepository.VerifyUploadSchema(ctx, database); err != nil {
		return nil, err
	}
	privateStore, err := documentstorage.NewPrivateFileStore(settings.DocumentStorageRoot, settings.DocumentPublicRoot)
	if err != nil {
		return nil, err
	}
	malwareScanner, err := documentstorage.NewClamAVScanner(settings.DocumentClamAVAddress, 5*time.Second)
	if err != nil {
		return nil, err
	}
	ingestor, err := documentstorage.NewIngestor(privateStore, malwareScanner)
	if err != nil {
		return nil, err
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	uploadRepository := documentrepository.NewMySQLRepository(database)
	uploadService := documents.NewUploadService(accessService, uploadRepository, ingestor)
	uploadController := documentcontroller.NewUploadController(uploadService)
	return httpapi.AuthenticatedRouteModule(verifier, uploadController), nil
}

func callCenterDocumentDownloadRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterDocumentDownloadAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errDocumentDownloadRequiresDatabase
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := documentrepository.VerifyUploadSchema(ctx, database); err != nil {
		return nil, err
	}
	privateStore, err := documentstorage.NewPrivateFileStore(settings.DocumentStorageRoot, settings.DocumentPublicRoot)
	if err != nil {
		return nil, err
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	documentRepository := documentrepository.NewMySQLRepository(database)
	downloadService := documents.NewDownloadService(accessService, documentRepository, privateStore)
	downloadController := documentcontroller.NewDownloadController(downloadService)
	return httpapi.AuthenticatedRouteModule(verifier, downloadController), nil
}

func callCenterDocumentPreviewRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterDocumentPreviewAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errDocumentPreviewRequiresDatabase
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := documentrepository.VerifyUploadSchema(ctx, database); err != nil {
		return nil, err
	}
	privateStore, err := documentstorage.NewPrivateFileStore(settings.DocumentStorageRoot, settings.DocumentPublicRoot)
	if err != nil {
		return nil, err
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	documentRepository := documentrepository.NewMySQLRepository(database)
	downloadService := documents.NewDownloadService(accessService, documentRepository, privateStore)
	previewController := documentcontroller.NewPreviewController(downloadService)
	return httpapi.AuthenticatedRouteModule(verifier, previewController), nil
}

func callCenterDocumentDeleteRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterDocumentDeleteAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errDocumentDeleteRequiresDatabase
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := documentrepository.VerifyUploadSchema(ctx, database); err != nil {
		return nil, err
	}
	privateStore, err := documentstorage.NewPrivateFileStore(settings.DocumentStorageRoot, settings.DocumentPublicRoot)
	if err != nil {
		return nil, err
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	documentRepository := documentrepository.NewMySQLRepository(database)
	deleteService := documents.NewDeleteService(accessService, documentRepository, privateStore)
	deleteController := documentcontroller.NewDeleteController(deleteService)
	return httpapi.AuthenticatedRouteModule(verifier, deleteController), nil
}

func callCenterCustomerSearchRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CallCenterCustomerSearchAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errCustomerSearchRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	customerRepository := customerrepository.NewMySQLCustomerRepository(database)
	customerSearchService := customers.NewCallCenterSearchService(accessService, customerRepository)
	customerController := customercontroller.New(customerSearchService)
	return httpapi.AuthenticatedRouteModule(verifier, customerController), nil
}

func customerProfileRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CustomerProfileAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errCustomerProfileRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	profileRepository := customerrepository.NewMySQLCustomerRepository(database)
	profileService := customers.NewProfileService(accessService, profileRepository)
	profileController := customercontroller.NewProfileController(profileService)
	return httpapi.AuthenticatedRouteModule(verifier, profileController), nil
}

func regionalAdminRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.RegionalAdminAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errRegionalAdminRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	regionalRepository := regionalrepository.NewMySQLRepository(database)
	regionalService := regionals.NewService(accessService, regionalRepository)
	regionalController := regionalcontroller.New(regionalService)
	return httpapi.AuthenticatedRouteModule(verifier, regionalController), nil
}

func csOfficerStatusRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.CSOfficerStatusAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errCSOfficerStatusRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	officerRepository := csofficerrepository.NewMySQLRepository(database)
	officerService := csofficers.NewService(accessService, officerRepository)
	officerController := csofficercontroller.New(officerService)
	return httpapi.AuthenticatedRouteModule(verifier, officerController), nil
}

func userLevelCatalogRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.UserLevelCatalogAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errUserLevelCatalogRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	catalogRepository := userlevelrepository.NewMySQLRepository(database)
	catalogService := userlevels.NewService(accessService, catalogRepository)
	catalogController := userlevelcontroller.New(catalogService)
	return httpapi.AuthenticatedRouteModule(verifier, catalogController), nil
}

func breadcrumbLinksRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.BreadcrumbLinksAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errBreadcrumbLinksRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	linksRepository := breadcrumbrepository.NewMySQLRepository(database)
	linksService := breadcrumbs.NewService(accessService, linksRepository)
	linksController := breadcrumbcontroller.New(linksService)
	return httpapi.AuthenticatedRouteModule(verifier, linksController), nil
}

func complaintTypeMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.ComplaintTypeMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errComplaintTypeMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := complainttyperepository.NewMySQLRepository(database)
	masterService := complainttypes.NewService(accessService, masterRepository)
	masterController := complainttypecontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func complaintSourceMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.ComplaintSourceMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errComplaintSourceMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := sourcerepository.NewMySQLRepository(database)
	masterService := sources.NewService(accessService, masterRepository)
	masterController := sourcecontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func shippingServiceMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.ShippingServiceMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errShippingServiceMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := shippingservicerepository.NewMySQLRepository(database)
	masterService := shippingservices.NewService(accessService, masterRepository)
	masterController := shippingservicecontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func documentTypeMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.DocumentTypeMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errDocumentTypeMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := documenttyperepository.NewMySQLRepository(database)
	masterService := documenttypes.NewService(accessService, masterRepository)
	masterController := documenttypecontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func handlingStatusMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.HandlingStatusMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errHandlingStatusMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := handlingstatusrepository.NewMySQLRepository(database)
	masterService := handlingstatuses.NewService(accessService, masterRepository)
	masterController := handlingstatuscontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func officeMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.OfficeMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errOfficeMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := officerepository.NewMySQLRepository(database)
	masterService := offices.NewService(accessService, masterRepository)
	masterController := officecontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func followUpStatusMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.FollowUpStatusMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errFollowUpStatusMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := followupstatusrepository.NewMySQLRepository(database)
	masterService := followupstatuses.NewService(accessService, masterRepository)
	masterController := followupstatuscontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func helpCategoryMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.HelpCategoryMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errHelpCategoryMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := helpcategoryrepository.NewMySQLRepository(database)
	masterService := helpcategories.NewService(accessService, masterRepository)
	masterController := helpcategorycontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func holidayCalendarMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.HolidayCalendarMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errHolidayCalendarMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := holidaycalendarrepository.NewMySQLRepository(database)
	masterService := holidaycalendar.NewService(accessService, masterRepository)
	masterController := holidaycalendarcontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func timezoneMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.TimezoneMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errTimezoneMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := timezonemasterrepository.NewMySQLRepository(database)
	masterService := timezonemaster.NewService(accessService, masterRepository)
	masterController := timezonemastercontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func languagesMasterRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.LanguagesMasterAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errLanguagesMasterRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := languagemasterrepository.NewMySQLRepository(database)
	masterService := languagemaster.NewService(accessService, masterRepository)
	masterController := languagemastercontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func appSettingsRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.AppSettingsAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errAppSettingsRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	masterRepository := appsettingsrepository.NewMySQLRepository(database)
	masterService := appsettings.NewService(accessService, masterRepository)
	masterController := appsettingscontroller.New(masterService)
	return httpapi.AuthenticatedRouteModule(verifier, masterController), nil
}

func helpArticleRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.HelpArticleAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errHelpArticleRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	articleRepository := helprepository.NewMySQLRepository(database)
	articleService := help.NewService(accessService, articleRepository)
	articleController := helparticlecontroller.New(articleService)
	return httpapi.AuthenticatedRouteModule(verifier, articleController), nil
}

func announcementRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.AnnouncementAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errAnnouncementRouteRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	announcementRepository := announcementrepository.NewMySQLRepository(database)
	announcementService := announcements.NewService(accessService, announcementRepository)
	announcementController := announcementcontroller.New(announcementService)
	return httpapi.AuthenticatedRouteModule(verifier, announcementController), nil
}

func guestIntakeSessionRoutes(settings config.GuestIntakeSessionSettings, staffJWT config.JWTSettings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.Enabled {
		return nil, nil
	}
	if database == nil {
		return nil, errGuestIntakeSessionRequiresDatabase
	}
	if settings.SigningKey == staffJWT.Secret {
		return nil, errGuestIntakeSessionKeyMustBeSeparate
	}
	tokenService, err := session.NewService(settings.SigningKey, settings.Issuer, settings.TokenLifetime)
	if err != nil {
		return nil, err
	}
	limiter, err := intakesessioncontroller.NewRateLimiter(5, time.Minute)
	if err != nil {
		return nil, err
	}
	controller, err := intakesessioncontroller.New(tokenService, settings.AllowedOrigins, limiter)
	if err != nil {
		return nil, err
	}
	return func(api fiber.Router) {
		controller.Register(api)
	}, nil
}

func guestIntakeSubmissionRoutes(settings config.GuestIntakeSubmissionSettings, sessionSettings config.GuestIntakeSessionSettings, staffJWT config.JWTSettings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.Enabled {
		return nil, nil
	}
	if database == nil {
		return nil, errGuestIntakeSubmissionRequiresDatabase
	}
	if !sessionSettings.Enabled {
		return nil, errGuestIntakeSubmissionRequiresSession
	}
	if sessionSettings.SigningKey == staffJWT.Secret {
		return nil, errGuestIntakeSubmissionKeyMustBeSeparate
	}
	conversationService, err := session.NewService(sessionSettings.SigningKey, sessionSettings.Issuer, sessionSettings.TokenLifetime)
	if err != nil {
		return nil, err
	}
	cchVerifier, err := authentication.NewJWTVerifier(staffJWT)
	if err != nil {
		return nil, err
	}
	ticketGateway, err := legacycch.NewClient(settings.CCHBaseURL, 5*time.Second)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	draftRepository := intakerepository.NewMySQLRepository(database)
	submissionService, err := submission.NewService(
		conversationService,
		cchVerifier,
		accessService,
		draftRepository,
		ticketGateway,
		submission.Settings{
			OriginOfficeCode: settings.OriginOfficeCode,
			SourceID:         7,
			ServiceUsername:  settings.CCHUsername,
			ServicePassword:  settings.CCHPassword,
		},
	)
	if err != nil {
		return nil, err
	}
	limiter, err := intakesessioncontroller.NewRateLimiter(5, time.Minute)
	if err != nil {
		return nil, err
	}
	controller, err := intakesubmissioncontroller.New(submissionService, sessionSettings.AllowedOrigins, limiter)
	if err != nil {
		return nil, err
	}
	return func(api fiber.Router) {
		controller.Register(api)
	}, nil
}

func guestIntakeReviewRoutes(settings config.Settings, database *sql.DB) (httpapi.VersionedRouteRegistrar, error) {
	if !settings.GuestIntakeReviewAPIEnabled {
		return nil, nil
	}
	if database == nil {
		return nil, errGuestIntakeReviewRequiresDatabase
	}
	verifier, err := authentication.NewJWTVerifier(settings.JWT)
	if err != nil {
		return nil, err
	}
	accessRepository := authorizationrepository.NewMySQLAccessRepository(database)
	accessService := authorization.NewAccessService(accessRepository)
	draftRepository := intakereviewrepository.NewMySQLRepository(database)
	reviewService, err := review.NewService(accessService, draftRepository, settings.GuestIntakeSubmission.OriginOfficeCode)
	if err != nil {
		return nil, err
	}
	ticketGateway, err := legacycch.NewClient(settings.GuestIntakeSubmission.CCHBaseURL, 5*time.Second)
	if err != nil {
		return nil, err
	}
	stateRepository := intakerepository.NewMySQLRepository(database)
	ticketCreator, err := review.NewTicketCreator(reviewService, draftRepository, stateRepository, verifier, ticketGateway, review.TicketSettings{
		OriginOfficeCode: settings.GuestIntakeSubmission.OriginOfficeCode,
		SourceID:         7,
		ServiceUsername:  settings.GuestIntakeSubmission.CCHUsername,
		ServicePassword:  settings.GuestIntakeSubmission.CCHPassword,
	})
	if err != nil {
		return nil, err
	}
	controller := intakereviewcontroller.New(reviewService, ticketCreator)
	return httpapi.AuthenticatedRouteModule(verifier, controller), nil
}
