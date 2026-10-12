package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaultAddress(t *testing.T) {
	t.Setenv("CCH_BE_ADDR", "")
	t.Setenv("CCH_DB_HOST", "")
	t.Setenv("CCH_DB_PORT", "")
	t.Setenv("CCH_DB_USER", "")
	t.Setenv("CCH_DB_PASSWORD", "")
	t.Setenv("CCH_DB_NAME", "")
	t.Setenv("CCH_JWT_SECRET", "")
	t.Setenv("CCH_JWT_ISSUER", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_STATUS_WRITE_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_FOLLOWUP_PLAN_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_FOLLOWUP_REALIZE_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_FOLLOWUP_CANCEL_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_FOLLOWUP_EDIT_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_METADATA_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_UPLOAD_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_DOWNLOAD_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_PREVIEW_API", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_DELETE_API", "")
	t.Setenv("CCH_DOCUMENT_STORAGE_ROOT", "")
	t.Setenv("CCH_DOCUMENT_PUBLIC_ROOT", "")
	t.Setenv("CCH_DOCUMENT_CLAMAV_ADDRESS", "")
	t.Setenv("CCH_LEGACY_STATUS_WRITES_FENCED", "")
	t.Setenv("CCH_ENABLE_CALL_CENTER_CUSTOMER_SEARCH_API", "")
	t.Setenv("CCH_ENABLE_CUSTOMER_PROFILE_API", "")
	t.Setenv("CCH_ENABLE_COMPLAINT_TYPE_MASTER_API", "")
	t.Setenv("CCH_ENABLE_COMPLAINT_SOURCE_MASTER_API", "")
	t.Setenv("CCH_ENABLE_SHIPPING_SERVICE_MASTER_API", "")
	t.Setenv("CCH_ENABLE_DOCUMENT_TYPE_MASTER_API", "")
	t.Setenv("CCH_ENABLE_HANDLING_STATUS_MASTER_API", "")
	t.Setenv("CCH_ENABLE_OFFICE_MASTER_API", "")
	t.Setenv("CCH_ENABLE_GUEST_INTAKE_REVIEW_API", "")
	t.Setenv("CCH_ENABLE_FOLLOWUP_STATUS_MASTER_API", "")
	t.Setenv("CCH_ENABLE_HELP_CATEGORY_MASTER_API", "")
	t.Setenv("CCH_ENABLE_HELP_ARTICLE_API", "")
	t.Setenv("CCH_ENABLE_ANNOUNCEMENT_API", "")
	t.Setenv("CCH_ENABLE_GUEST_INTAKE_SESSION_API", "")
	t.Setenv("CCH_INTAKE_SESSION_SECRET", "")
	t.Setenv("CCH_INTAKE_SESSION_ISSUER", "")
	t.Setenv("CCH_INTAKE_SESSION_TTL", "")
	t.Setenv("CCH_INTAKE_ALLOWED_ORIGINS", "")
	t.Setenv("CCH_ENABLE_GUEST_INTAKE_SUBMISSION_API", "")
	t.Setenv("CCH_INTAKE_CCH_API_BASE_URL", "")
	t.Setenv("CCH_INTAKE_CCH_USERNAME", "")
	t.Setenv("CCH_INTAKE_CCH_PASSWORD", "")
	t.Setenv("CCH_INTAKE_OFFICE_CODE", "")

	settings := Load()
	if settings.HTTPAddress != defaultHTTPAddress {
		t.Fatalf("HTTPAddress = %q, want %q", settings.HTTPAddress, defaultHTTPAddress)
	}
	if settings.MySQL.Port != defaultMySQLPort {
		t.Fatalf("MySQL.Port = %q, want %q", settings.MySQL.Port, defaultMySQLPort)
	}
	if settings.MySQL.Configured() {
		t.Fatal("MySQL should not be configured without credentials")
	}
	if settings.JWT.Configured() {
		t.Fatal("JWT should not be configured without a secret and issuer")
	}
	if settings.CallCenterComplaintReadAPIEnabled {
		t.Fatal("Call Center complaint API must be disabled by default")
	}
	if settings.CallCenterStatusWriteAPIEnabled || settings.CallCenterFollowUpPlanAPIEnabled || settings.CallCenterFollowUpRealizeAPIEnabled || settings.CallCenterFollowUpCancelAPIEnabled || settings.CallCenterFollowUpEditAPIEnabled || settings.CallCenterDocumentMetadataAPIEnabled || settings.CallCenterDocumentUploadAPIEnabled || settings.CallCenterDocumentDownloadAPIEnabled || settings.CallCenterDocumentPreviewAPIEnabled || settings.CallCenterDocumentDeleteAPIEnabled || settings.LegacyStatusWritesFenced {
		t.Fatal("Call Center write APIs must be disabled by default")
	}
	if settings.DocumentStorageRoot != "" || settings.DocumentPublicRoot != "" || settings.DocumentClamAVAddress != "" {
		t.Fatalf("document upload infrastructure must be unconfigured by default: %#v", settings)
	}
	if settings.CallCenterCustomerSearchAPIEnabled {
		t.Fatal("Call Center customer search API must be disabled by default")
	}
	if settings.CustomerProfileAPIEnabled {
		t.Fatal("customer profile CRUD API must be disabled by default")
	}
	if settings.ComplaintTypeMasterAPIEnabled {
		t.Fatal("complaint type master API must be disabled by default")
	}
	if settings.ComplaintSourceMasterAPIEnabled {
		t.Fatal("complaint source master API must be disabled by default")
	}
	if settings.ShippingServiceMasterAPIEnabled {
		t.Fatal("shipping service master API must be disabled by default")
	}
	if settings.OfficeMasterAPIEnabled {
		t.Fatal("office master API must be disabled by default")
	}
	if settings.FollowUpStatusMasterAPIEnabled {
		t.Fatal("follow-up status master API must be disabled by default")
	}
	if settings.HelpCategoryMasterAPIEnabled {
		t.Fatal("help category master API must be disabled by default")
	}
	if settings.HelpArticleAPIEnabled {
		t.Fatal("help article API must be disabled by default")
	}
	if settings.AnnouncementAPIEnabled {
		t.Fatal("announcement API must be disabled by default")
	}
	if settings.GuestIntakeReviewAPIEnabled {
		t.Fatal("guest intake review API must be disabled by default")
	}
	if settings.GuestIntakeSession.Enabled || len(settings.GuestIntakeSession.AllowedOrigins) != 0 || settings.GuestIntakeSession.TokenLifetime != 0 {
		t.Fatalf("guest intake session must be disabled and unconfigured by default: %#v", settings.GuestIntakeSession)
	}
	if settings.GuestIntakeSubmission.Enabled || settings.GuestIntakeSubmission.CCHPassword != "" {
		t.Fatalf("guest intake submission must be disabled and unconfigured by default: %#v", settings.GuestIntakeSubmission)
	}
}

func TestLoadDocumentUploadConfiguration(t *testing.T) {
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_UPLOAD_API", "true")
	t.Setenv("CCH_DOCUMENT_STORAGE_ROOT", "C:\\private\\cch-documents")
	t.Setenv("CCH_DOCUMENT_PUBLIC_ROOT", "C:\\laragon\\www\\cch")
	t.Setenv("CCH_DOCUMENT_CLAMAV_ADDRESS", "127.0.0.1:3310")
	settings := Load()
	if !settings.CallCenterDocumentUploadAPIEnabled || settings.DocumentStorageRoot != "C:\\private\\cch-documents" ||
		settings.DocumentPublicRoot != "C:\\laragon\\www\\cch" || settings.DocumentClamAVAddress != "127.0.0.1:3310" {
		t.Fatalf("document upload settings = %#v", settings)
	}
}

func TestLoadDocumentDownloadConfiguration(t *testing.T) {
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_DOWNLOAD_API", "true")
	settings := Load()
	if !settings.CallCenterDocumentDownloadAPIEnabled {
		t.Fatal("document download flag should be enabled for true")
	}
}

func TestLoadDocumentDeleteConfiguration(t *testing.T) {
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_DELETE_API", "true")
	settings := Load()
	if !settings.CallCenterDocumentDeleteAPIEnabled {
		t.Fatal("document delete flag should be enabled for true")
	}
}

func TestLoadDocumentPreviewConfiguration(t *testing.T) {
	t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_PREVIEW_API", "true")
	settings := Load()
	if !settings.CallCenterDocumentPreviewAPIEnabled {
		t.Fatal("document preview flag should be enabled for true")
	}
}

func TestLoadEnablesCallCenterDocumentPreviewAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_PREVIEW_API", test.value)
		if got := Load().CallCenterDocumentPreviewAPIEnabled; got != test.want {
			t.Errorf("document preview flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCallCenterDocumentDeleteAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_DELETE_API", test.value)
		if got := Load().CallCenterDocumentDeleteAPIEnabled; got != test.want {
			t.Errorf("document delete flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCallCenterDocumentMetadataAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_DOCUMENT_METADATA_API", test.value)
		if got := Load().CallCenterDocumentMetadataAPIEnabled; got != test.want {
			t.Errorf("document metadata flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCallCenterFollowUpPlanAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_FOLLOWUP_PLAN_API", test.value)
		if got := Load().CallCenterFollowUpPlanAPIEnabled; got != test.want {
			t.Errorf("follow-up plan flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCallCenterFollowUpRealizeAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_FOLLOWUP_REALIZE_API", test.value)
		if got := Load().CallCenterFollowUpRealizeAPIEnabled; got != test.want {
			t.Errorf("follow-up realization flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadStatusMutationGatesRequireTrueBooleans(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_STATUS_WRITE_API", test.value)
		if got := Load().CallCenterStatusWriteAPIEnabled; got != test.want {
			t.Errorf("status write flag %q = %v, want %v", test.value, got, test.want)
		}
		t.Setenv("CCH_LEGACY_STATUS_WRITES_FENCED", test.value)
		if got := Load().LegacyStatusWritesFenced; got != test.want {
			t.Errorf("legacy status fence %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesGuestIntakeReviewAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_GUEST_INTAKE_REVIEW_API", test.value)
		if got := Load().GuestIntakeReviewAPIEnabled; got != test.want {
			t.Fatalf("guest intake review flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCallCenterComplaintAPIOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API", test.value)
		if got := Load().CallCenterComplaintReadAPIEnabled; got != test.want {
			t.Fatalf("feature flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCallCenterCustomerSearchOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CALL_CENTER_CUSTOMER_SEARCH_API", test.value)
		if got := Load().CallCenterCustomerSearchAPIEnabled; got != test.want {
			t.Fatalf("customer search feature flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesHelpArticleOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_HELP_ARTICLE_API", test.value)
		if got := Load().HelpArticleAPIEnabled; got != test.want {
			t.Fatalf("help article flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesHelpCategoryMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_HELP_CATEGORY_MASTER_API", test.value)
		if got := Load().HelpCategoryMasterAPIEnabled; got != test.want {
			t.Fatalf("help category master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesHolidayCalendarMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_HOLIDAY_CALENDAR_MASTER_API", test.value)
		if got := Load().HolidayCalendarMasterAPIEnabled; got != test.want {
			t.Fatalf("holiday calendar master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesTimezoneMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_TIMEZONE_MASTER_API", test.value)
		if got := Load().TimezoneMasterAPIEnabled; got != test.want {
			t.Fatalf("timezone master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesLanguagesMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_LANGUAGES_MASTER_API", test.value)
		if got := Load().LanguagesMasterAPIEnabled; got != test.want {
			t.Fatalf("languages master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesAppSettingsOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_APP_SETTINGS_API", test.value)
		if got := Load().AppSettingsAPIEnabled; got != test.want {
			t.Fatalf("app settings flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesRegionalAdminOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true}, {value: "1", want: true},
		{value: "false", want: false}, {value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_REGIONAL_ADMIN_API", test.value)
		if got := Load().RegionalAdminAPIEnabled; got != test.want {
			t.Fatalf("regional admin flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesCSOfficerStatusOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true}, {value: "1", want: true},
		{value: "false", want: false}, {value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_CS_OFFICER_STATUS_API", test.value)
		if got := Load().CSOfficerStatusAPIEnabled; got != test.want {
			t.Fatalf("CS Officer status flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesUserLevelCatalogOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true}, {value: "1", want: true},
		{value: "false", want: false}, {value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_USER_LEVEL_CATALOG_API", test.value)
		if got := Load().UserLevelCatalogAPIEnabled; got != test.want {
			t.Fatalf("user-level catalog flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesBreadcrumbLinksOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true}, {value: "1", want: true},
		{value: "false", want: false}, {value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_BREADCRUMB_LINKS_API", test.value)
		if got := Load().BreadcrumbLinksAPIEnabled; got != test.want {
			t.Fatalf("breadcrumb links flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesComplaintTypeMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_COMPLAINT_TYPE_MASTER_API", test.value)
		if got := Load().ComplaintTypeMasterAPIEnabled; got != test.want {
			t.Fatalf("complaint type master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesComplaintSourceMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_COMPLAINT_SOURCE_MASTER_API", test.value)
		if got := Load().ComplaintSourceMasterAPIEnabled; got != test.want {
			t.Fatalf("complaint source master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadEnablesShippingServiceMasterOnlyForTrueBoolean(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "enabled", want: false},
	} {
		t.Setenv("CCH_ENABLE_SHIPPING_SERVICE_MASTER_API", test.value)
		if got := Load().ShippingServiceMasterAPIEnabled; got != test.want {
			t.Fatalf("shipping service master flag %q = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestLoadGuestIntakeSessionSettings(t *testing.T) {
	t.Setenv("CCH_ENABLE_GUEST_INTAKE_SESSION_API", "true")
	t.Setenv("CCH_INTAKE_SESSION_SECRET", "separate-test-only-intake-session-secret")
	t.Setenv("CCH_INTAKE_SESSION_ISSUER", "cch-intake-test")
	t.Setenv("CCH_INTAKE_SESSION_TTL", "20m")
	t.Setenv("CCH_INTAKE_ALLOWED_ORIGINS", "https://chat.example.test, https://portal.example.test ,, ")

	settings := Load().GuestIntakeSession
	if !settings.Enabled || settings.SigningKey != "separate-test-only-intake-session-secret" || settings.Issuer != "cch-intake-test" || settings.TokenLifetime != 20*time.Minute {
		t.Fatalf("unexpected guest intake session settings: %#v", settings)
	}
	if len(settings.AllowedOrigins) != 2 || settings.AllowedOrigins[0] != "https://chat.example.test" || settings.AllowedOrigins[1] != "https://portal.example.test" {
		t.Fatalf("unexpected allowed origins: %#v", settings.AllowedOrigins)
	}
}

func TestLoadInvalidGuestIntakeLifetimeFailsClosed(t *testing.T) {
	t.Setenv("CCH_ENABLE_GUEST_INTAKE_SESSION_API", "true")
	for _, raw := range []string{"", "not-a-duration", "0s"} {
		t.Setenv("CCH_INTAKE_SESSION_TTL", raw)
		if got := Load().GuestIntakeSession.TokenLifetime; got != 0 {
			t.Errorf("lifetime %q = %v, want zero so route configuration fails closed", raw, got)
		}
	}
}

func TestLoadGuestIntakeSubmissionSettings(t *testing.T) {
	t.Setenv("CCH_ENABLE_GUEST_INTAKE_SUBMISSION_API", "true")
	t.Setenv("CCH_INTAKE_CCH_API_BASE_URL", "https://legacy-cch.example.test")
	t.Setenv("CCH_INTAKE_CCH_USERNAME", "intake-bot")
	t.Setenv("CCH_INTAKE_CCH_PASSWORD", "test-only-password")
	t.Setenv("CCH_INTAKE_OFFICE_CODE", "40005CC")

	settings := Load().GuestIntakeSubmission
	if !settings.Enabled || settings.CCHBaseURL != "https://legacy-cch.example.test" || settings.CCHUsername != "intake-bot" || settings.OriginOfficeCode != "40005CC" {
		t.Fatalf("unexpected guest intake submission settings: %#v", settings)
	}
}

func TestLoadUsesConfiguredAddress(t *testing.T) {
	const address = "127.0.0.1:8090"
	t.Setenv("CCH_BE_ADDR", address)

	settings := Load()
	if settings.HTTPAddress != address {
		t.Fatalf("HTTPAddress = %q, want %q", settings.HTTPAddress, address)
	}
}

func TestLoadUsesConfiguredMySQLSettings(t *testing.T) {
	t.Setenv("CCH_DB_HOST", "127.0.0.1")
	t.Setenv("CCH_DB_PORT", "3307")
	t.Setenv("CCH_DB_USER", "cch_app_test")
	t.Setenv("CCH_DB_PASSWORD", "test-only-password")
	t.Setenv("CCH_DB_NAME", "cch_test")

	settings := Load()
	if !settings.MySQL.Configured() {
		t.Fatal("MySQL settings should be configured")
	}
	if settings.MySQL.Host != "127.0.0.1" || settings.MySQL.Port != "3307" || settings.MySQL.User != "cch_app_test" || settings.MySQL.Database != "cch_test" {
		t.Fatalf("unexpected MySQL settings: %#v", settings.MySQL)
	}
}

func TestLoadUsesConfiguredJWTSettings(t *testing.T) {
	t.Setenv("CCH_JWT_SECRET", "test-only-signing-key-that-is-long-enough")
	t.Setenv("CCH_JWT_ISSUER", "cch-test")

	settings := Load()
	if !settings.JWT.Configured() {
		t.Fatal("JWT settings should be configured")
	}
	if settings.JWT.Issuer != "cch-test" {
		t.Fatalf("JWT.Issuer = %q, want %q", settings.JWT.Issuer, "cch-test")
	}
}
