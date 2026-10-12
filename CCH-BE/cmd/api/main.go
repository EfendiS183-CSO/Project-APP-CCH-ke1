package main

import (
	"database/sql"
	"log"

	"cch.local/backend/internal/modules/health"
	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/database"
	"cch.local/backend/internal/platform/envfile"
	"cch.local/backend/internal/platform/httpapi"
)

func main() {
	// Development convenience: load .env when the process environment has not
	// been populated yet. Existing variables always win, so production secrets
	// injected through the environment are never overridden.
	if err := envfile.LoadFirst(".env", "../.env", "../../.env"); err != nil {
		log.Print("load .env: ", err)
	}

	settings := config.Load()
	var mysqlReadiness health.ReadinessChecker
	var mysqlConnection *sql.DB
	if settings.MySQL.Configured() {
		connection, err := database.OpenMySQL(settings.MySQL)
		if err != nil {
			log.Fatal("configure MySQL connection: ", err)
		}
		defer connection.Close()
		mysqlConnection = connection
		mysqlReadiness = connection
	} else {
		log.Print("MySQL is not configured; /readyz will report not ready")
	}

	routeRegistrar, err := callCenterComplaintRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center complaint routes: ", err)
	}
	statusWriteRegistrar, err := callCenterStatusWriteRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center status mutation route: ", err)
	}
	followUpPlanRegistrar, err := callCenterFollowUpPlanRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center follow-up plan route: ", err)
	}
	followUpRealizationRegistrar, err := callCenterFollowUpRealizationRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center follow-up realization route: ", err)
	}
	followUpCancelRegistrar, err := callCenterFollowUpCancelRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center follow-up cancellation route: ", err)
	}
	followUpEditRegistrar, err := callCenterFollowUpEditRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center follow-up edit route: ", err)
	}
	documentMetadataRegistrar, err := callCenterDocumentMetadataRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center document metadata route: ", err)
	}
	documentUploadRegistrar, err := callCenterDocumentUploadRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center document upload route: ", err)
	}
	documentDownloadRegistrar, err := callCenterDocumentDownloadRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center document download route: ", err)
	}
	documentPreviewRegistrar, err := callCenterDocumentPreviewRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center document preview route: ", err)
	}
	documentDeleteRegistrar, err := callCenterDocumentDeleteRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center document delete route: ", err)
	}
	customerSearchRegistrar, err := callCenterCustomerSearchRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Call Center customer search route: ", err)
	}
	customerProfileRegistrar, err := customerProfileRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure customer profile routes: ", err)
	}
	regionalAdminRegistrar, err := regionalAdminRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure Regional admin routes: ", err)
	}
	csOfficerStatusRegistrar, err := csOfficerStatusRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure CS Officer status routes: ", err)
	}
	userLevelCatalogRegistrar, err := userLevelCatalogRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure user-level catalog routes: ", err)
	}
	breadcrumbLinksRegistrar, err := breadcrumbLinksRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure breadcrumb links routes: ", err)
	}
	complaintTypeMasterRegistrar, err := complaintTypeMasterRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure complaint type master route: ", err)
	}
	complaintSourceMasterRegistrar, err := complaintSourceMasterRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure complaint source master route: ", err)
	}
	shippingServiceMasterRegistrar, err := shippingServiceMasterRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure shipping service master route: ", err)
	}
	documentTypeMasterRegistrar := configureDocumentTypeMasterRoutes(settings, mysqlConnection)
	handlingStatusMasterRegistrar := configureHandlingStatusMasterRoutes(settings, mysqlConnection)
	officeMasterRegistrar := configureOfficeMasterRoutes(settings, mysqlConnection)
	followUpStatusMasterRegistrar := configureFollowUpStatusMasterRoutes(settings, mysqlConnection)
	helpCategoryMasterRegistrar, err := helpCategoryMasterRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure help category master route: ", err)
	}
	holidayCalendarMasterRegistrar := configureHolidayCalendarMasterRoutes(settings, mysqlConnection)
	timezoneMasterRegistrar := configureTimezoneMasterRoutes(settings, mysqlConnection)
	languagesMasterRegistrar := configureLanguagesMasterRoutes(settings, mysqlConnection)
	appSettingsRegistrar := configureAppSettingsRoutes(settings, mysqlConnection)
	helpArticleRegistrar, err := helpArticleRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure help article routes: ", err)
	}
	announcementRegistrar, err := announcementRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure announcement route: ", err)
	}
	guestIntakeRegistrar, err := guestIntakeSessionRoutes(settings.GuestIntakeSession, settings.JWT, mysqlConnection)
	if err != nil {
		log.Fatal("configure guest intake session route: ", err)
	}
	guestIntakeSubmissionRegistrar, err := guestIntakeSubmissionRoutes(settings.GuestIntakeSubmission, settings.GuestIntakeSession, settings.JWT, mysqlConnection)
	if err != nil {
		log.Fatal("configure guest intake submission route: ", err)
	}
	guestIntakeReviewRegistrar, err := guestIntakeReviewRoutes(settings, mysqlConnection)
	if err != nil {
		log.Fatal("configure guest intake review routes: ", err)
	}
	if routeRegistrar != nil {
		log.Print("Call Center complaint read routes enabled; use only after coordinated legacy key rotation")
	} else {
		log.Print("Call Center complaint read routes are disabled")
	}
	if statusWriteRegistrar != nil {
		log.Print("Call Center status mutation enabled; legacy status Add must remain fenced")
	} else {
		log.Print("Call Center status mutation is disabled")
	}
	if followUpPlanRegistrar != nil {
		log.Print("Call Center follow-up plan creation enabled")
	} else {
		log.Print("Call Center follow-up plan creation is disabled")
	}
	if followUpRealizationRegistrar != nil {
		log.Print("Call Center follow-up realization enabled")
	} else {
		log.Print("Call Center follow-up realization is disabled")
	}
	if followUpCancelRegistrar != nil {
		log.Print("Call Center follow-up cancellation enabled")
	} else {
		log.Print("Call Center follow-up cancellation is disabled")
	}
	if followUpEditRegistrar != nil {
		log.Print("Call Center follow-up edit enabled")
	} else {
		log.Print("Call Center follow-up edit is disabled")
	}
	if documentMetadataRegistrar != nil {
		log.Print("Call Center document metadata listing enabled; file content and storage paths are not exposed")
	} else {
		log.Print("Call Center document metadata listing is disabled")
	}
	if documentUploadRegistrar != nil {
		log.Print("Call Center document upload enabled; private storage and ClamAV are configured")
	} else {
		log.Print("Call Center document upload is disabled")
	}
	if documentDownloadRegistrar != nil {
		log.Print("Call Center document download enabled; private storage is configured")
	} else {
		log.Print("Call Center document download is disabled")
	}
	if documentPreviewRegistrar != nil {
		log.Print("Call Center document preview enabled; verified bytes are served inline")
	} else {
		log.Print("Call Center document preview is disabled")
	}
	if documentDeleteRegistrar != nil {
		log.Print("Call Center document delete enabled; office-owned metadata rows are removed transactionally")
	} else {
		log.Print("Call Center document delete is disabled")
	}
	if customerSearchRegistrar != nil {
		log.Print("Call Center customer profile search enabled; this exposes scoped customer PII")
	} else {
		log.Print("Call Center customer profile search is disabled")
	}
	if regionalAdminRegistrar != nil {
		log.Print("Regional office administration enabled; office code remains immutable")
	} else {
		log.Print("Regional office administration is disabled")
	}
	if csOfficerStatusRegistrar != nil {
		log.Print("CS Officer roster/status read enabled; contact fields require cch_cs_officer ACL")
	} else {
		log.Print("CS Officer roster/status read is disabled")
	}
	if userLevelCatalogRegistrar != nil {
		log.Print("User-level catalog read enabled; role permission masks are not exposed")
	} else {
		log.Print("User-level catalog read is disabled")
	}
	if breadcrumbLinksRegistrar != nil {
		log.Print("Breadcrumb links administration enabled; nested-set columns are updated transactionally")
	} else {
		log.Print("Breadcrumb links administration is disabled")
	}
	if complaintTypeMasterRegistrar != nil {
		log.Print("Complaint type master CRUD enabled; action permissions are revalidated from CCH")
	} else {
		log.Print("Complaint type master CRUD is disabled")
	}
	if complaintSourceMasterRegistrar != nil {
		log.Print("Complaint source master CRUD enabled; action permissions are revalidated from CCH")
	} else {
		log.Print("Complaint source master CRUD is disabled")
	}
	if shippingServiceMasterRegistrar != nil {
		log.Print("Shipping service master CRUD enabled; action permissions are revalidated from CCH")
	} else {
		log.Print("Shipping service master CRUD is disabled")
	}
	if guestIntakeRegistrar != nil {
		log.Print("Guest intake session issuance enabled; verify origin allowlist and edge rate limiting")
	} else {
		log.Print("Guest intake session issuance is disabled")
	}
	if guestIntakeSubmissionRegistrar != nil {
		log.Print("Guest intake submission enabled; CCH technical account and ticket reconciliation are active")
	} else {
		log.Print("Guest intake submission is disabled")
	}
	if guestIntakeReviewRegistrar != nil {
		log.Print("Guest intake CS review API enabled; reviewer role, office, and permissions are revalidated")
	} else {
		log.Print("Guest intake CS review API is disabled")
	}

	server := httpapi.NewServer(
		mysqlReadiness,
		routeRegistrar,
		statusWriteRegistrar,
		followUpPlanRegistrar,
		followUpRealizationRegistrar,
		followUpCancelRegistrar,
		followUpEditRegistrar,
		documentMetadataRegistrar,
		documentUploadRegistrar,
		documentDownloadRegistrar,
		documentPreviewRegistrar,
		documentDeleteRegistrar,
		customerSearchRegistrar,
		customerProfileRegistrar,
		regionalAdminRegistrar,
		csOfficerStatusRegistrar,
		userLevelCatalogRegistrar,
		breadcrumbLinksRegistrar,
		complaintTypeMasterRegistrar,
		complaintSourceMasterRegistrar,
		shippingServiceMasterRegistrar,
		documentTypeMasterRegistrar,
		handlingStatusMasterRegistrar,
		officeMasterRegistrar,
		followUpStatusMasterRegistrar,
		helpCategoryMasterRegistrar,
		holidayCalendarMasterRegistrar,
		timezoneMasterRegistrar,
		languagesMasterRegistrar,
		appSettingsRegistrar,
		helpArticleRegistrar,
		announcementRegistrar,
		guestIntakeRegistrar,
		guestIntakeSubmissionRegistrar,
		guestIntakeReviewRegistrar,
	)

	log.Printf("CCH backend listening on %s", settings.HTTPAddress)
	if err := server.Listen(settings.HTTPAddress); err != nil {
		log.Fatal(err)
	}
}

func configureDocumentTypeMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := documentTypeMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure document type master route: ", err)
	}
	if registrar == nil {
		log.Print("Document type master CRUD is disabled")
	} else {
		log.Print("Document type master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureFollowUpStatusMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := followUpStatusMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure follow-up status master route: ", err)
	}
	if registrar == nil {
		log.Print("Follow-up status master CRUD is disabled")
	} else {
		log.Print("Follow-up status master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureOfficeMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := officeMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure office master route: ", err)
	}
	if registrar == nil {
		log.Print("Office master CRUD is disabled")
	} else {
		log.Print("Office master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureTimezoneMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := timezoneMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure timezone master route: ", err)
	}
	if registrar == nil {
		log.Print("Timezone master CRUD is disabled")
	} else {
		log.Print("Timezone master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureLanguagesMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := languagesMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure languages master route: ", err)
	}
	if registrar == nil {
		log.Print("Languages master CRUD is disabled")
	} else {
		log.Print("Languages master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureAppSettingsRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := appSettingsRoutes(settings, database)
	if err != nil {
		log.Fatal("configure app settings route: ", err)
	}
	if registrar == nil {
		log.Print("App settings API is disabled")
	} else {
		log.Print("App settings API enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureHandlingStatusMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := handlingStatusMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure handling status master route: ", err)
	}
	if registrar == nil {
		log.Print("Handling status master CRUD is disabled")
	} else {
		log.Print("Handling status master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}

func configureHolidayCalendarMasterRoutes(settings config.Settings, database *sql.DB) httpapi.VersionedRouteRegistrar {
	registrar, err := holidayCalendarMasterRoutes(settings, database)
	if err != nil {
		log.Fatal("configure holiday calendar master route: ", err)
	}
	if registrar == nil {
		log.Print("Holiday calendar master CRUD is disabled")
	} else {
		log.Print("Holiday calendar master CRUD enabled; action permissions are revalidated from CCH")
	}
	return registrar
}
