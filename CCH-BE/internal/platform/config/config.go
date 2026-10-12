package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultHTTPAddress = ":8080"
const defaultMySQLPort = "3306"

type Settings struct {
	HTTPAddress                          string
	MySQL                                MySQLSettings
	JWT                                  JWTSettings
	CallCenterComplaintReadAPIEnabled    bool
	CallCenterStatusWriteAPIEnabled      bool
	CallCenterFollowUpPlanAPIEnabled     bool
	CallCenterFollowUpRealizeAPIEnabled  bool
	CallCenterFollowUpCancelAPIEnabled   bool
	CallCenterFollowUpEditAPIEnabled     bool
	CallCenterDocumentMetadataAPIEnabled bool
	CallCenterDocumentUploadAPIEnabled   bool
	CallCenterDocumentDownloadAPIEnabled bool
	CallCenterDocumentPreviewAPIEnabled  bool
	CallCenterDocumentDeleteAPIEnabled   bool
	DocumentStorageRoot                  string
	DocumentPublicRoot                   string
	DocumentClamAVAddress                string
	LegacyStatusWritesFenced             bool
	CallCenterCustomerSearchAPIEnabled   bool
	CustomerProfileAPIEnabled            bool
	ComplaintTypeMasterAPIEnabled        bool
	ComplaintSourceMasterAPIEnabled      bool
	ShippingServiceMasterAPIEnabled      bool
	DocumentTypeMasterAPIEnabled         bool
	HandlingStatusMasterAPIEnabled       bool
	OfficeMasterAPIEnabled               bool
	GuestIntakeReviewAPIEnabled          bool
	TimezoneMasterAPIEnabled             bool
	FollowUpStatusMasterAPIEnabled       bool
	HelpCategoryMasterAPIEnabled         bool
	HolidayCalendarMasterAPIEnabled      bool
	LanguagesMasterAPIEnabled            bool
	AppSettingsAPIEnabled                bool
	RegionalAdminAPIEnabled              bool
	CSOfficerStatusAPIEnabled            bool
	UserLevelCatalogAPIEnabled           bool
	BreadcrumbLinksAPIEnabled            bool
	HelpArticleAPIEnabled                bool
	AnnouncementAPIEnabled               bool
	GuestIntakeSession                   GuestIntakeSessionSettings
	GuestIntakeSubmission                GuestIntakeSubmissionSettings
}

type MySQLSettings struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type JWTSettings struct {
	Secret string
	Issuer string
}

type GuestIntakeSessionSettings struct {
	Enabled        bool
	SigningKey     string
	Issuer         string
	TokenLifetime  time.Duration
	AllowedOrigins []string
}

type GuestIntakeSubmissionSettings struct {
	Enabled          bool
	CCHBaseURL       string
	CCHUsername      string
	CCHPassword      string
	OriginOfficeCode string
}

func (settings JWTSettings) Configured() bool {
	return settings.Secret != "" && settings.Issuer != ""
}

func (settings MySQLSettings) Configured() bool {
	return settings.Host != "" && settings.Port != "" && settings.User != "" && settings.Database != "" &&
		(!settings.isRemote() || settings.Password != "")
}

func (settings MySQLSettings) isRemote() bool {
	return !strings.EqualFold(settings.Host, "localhost") && !strings.HasPrefix(settings.Host, "127.0.0.1")
}

func Load() Settings {
	address := os.Getenv("CCH_BE_ADDR")
	if address == "" {
		address = defaultHTTPAddress
	}

	port := os.Getenv("CCH_DB_PORT")
	if port == "" {
		port = defaultMySQLPort
	}

	intakeLifetime := time.Duration(0)
	if rawLifetime := strings.TrimSpace(os.Getenv("CCH_INTAKE_SESSION_TTL")); rawLifetime != "" {
		intakeLifetime, _ = time.ParseDuration(rawLifetime)
	}

	return Settings{
		HTTPAddress:                          address,
		CallCenterComplaintReadAPIEnabled:    envBool("CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API"),
		CallCenterStatusWriteAPIEnabled:      envBool("CCH_ENABLE_CALL_CENTER_STATUS_WRITE_API"),
		CallCenterFollowUpPlanAPIEnabled:     envBool("CCH_ENABLE_CALL_CENTER_FOLLOWUP_PLAN_API"),
		CallCenterFollowUpRealizeAPIEnabled:  envBool("CCH_ENABLE_CALL_CENTER_FOLLOWUP_REALIZE_API"),
		CallCenterFollowUpCancelAPIEnabled:   envBool("CCH_ENABLE_CALL_CENTER_FOLLOWUP_CANCEL_API"),
		CallCenterFollowUpEditAPIEnabled:     envBool("CCH_ENABLE_CALL_CENTER_FOLLOWUP_EDIT_API"),
		CallCenterDocumentMetadataAPIEnabled: envBool("CCH_ENABLE_CALL_CENTER_DOCUMENT_METADATA_API"),
		CallCenterDocumentUploadAPIEnabled:   envBool("CCH_ENABLE_CALL_CENTER_DOCUMENT_UPLOAD_API"),
		CallCenterDocumentDownloadAPIEnabled: envBool("CCH_ENABLE_CALL_CENTER_DOCUMENT_DOWNLOAD_API"),
		CallCenterDocumentPreviewAPIEnabled:  envBool("CCH_ENABLE_CALL_CENTER_DOCUMENT_PREVIEW_API"),
		CallCenterDocumentDeleteAPIEnabled:   envBool("CCH_ENABLE_CALL_CENTER_DOCUMENT_DELETE_API"),
		DocumentStorageRoot:                  strings.TrimSpace(os.Getenv("CCH_DOCUMENT_STORAGE_ROOT")),
		DocumentPublicRoot:                   strings.TrimSpace(os.Getenv("CCH_DOCUMENT_PUBLIC_ROOT")),
		DocumentClamAVAddress:                strings.TrimSpace(os.Getenv("CCH_DOCUMENT_CLAMAV_ADDRESS")),
		LegacyStatusWritesFenced:             envBool("CCH_LEGACY_STATUS_WRITES_FENCED"),
		CallCenterCustomerSearchAPIEnabled:   envBool("CCH_ENABLE_CALL_CENTER_CUSTOMER_SEARCH_API"),
		CustomerProfileAPIEnabled:            envBool("CCH_ENABLE_CUSTOMER_PROFILE_API"),
		ComplaintTypeMasterAPIEnabled:        envBool("CCH_ENABLE_COMPLAINT_TYPE_MASTER_API"),
		ComplaintSourceMasterAPIEnabled:      envBool("CCH_ENABLE_COMPLAINT_SOURCE_MASTER_API"),
		ShippingServiceMasterAPIEnabled:      envBool("CCH_ENABLE_SHIPPING_SERVICE_MASTER_API"),
		DocumentTypeMasterAPIEnabled:         envBool("CCH_ENABLE_DOCUMENT_TYPE_MASTER_API"),
		HandlingStatusMasterAPIEnabled:       envBool("CCH_ENABLE_HANDLING_STATUS_MASTER_API"),
		OfficeMasterAPIEnabled:               envBool("CCH_ENABLE_OFFICE_MASTER_API"),
		GuestIntakeReviewAPIEnabled:          envBool("CCH_ENABLE_GUEST_INTAKE_REVIEW_API"),
		TimezoneMasterAPIEnabled:             envBool("CCH_ENABLE_TIMEZONE_MASTER_API"),
		FollowUpStatusMasterAPIEnabled:       envBool("CCH_ENABLE_FOLLOWUP_STATUS_MASTER_API"),
		HelpCategoryMasterAPIEnabled:         envBool("CCH_ENABLE_HELP_CATEGORY_MASTER_API"),
		HolidayCalendarMasterAPIEnabled:      envBool("CCH_ENABLE_HOLIDAY_CALENDAR_MASTER_API"),
		LanguagesMasterAPIEnabled:            envBool("CCH_ENABLE_LANGUAGES_MASTER_API"),
		AppSettingsAPIEnabled:                envBool("CCH_ENABLE_APP_SETTINGS_API"),
		RegionalAdminAPIEnabled:              envBool("CCH_ENABLE_REGIONAL_ADMIN_API"),
		CSOfficerStatusAPIEnabled:            envBool("CCH_ENABLE_CS_OFFICER_STATUS_API"),
		UserLevelCatalogAPIEnabled:           envBool("CCH_ENABLE_USER_LEVEL_CATALOG_API"),
		BreadcrumbLinksAPIEnabled:            envBool("CCH_ENABLE_BREADCRUMB_LINKS_API"),
		HelpArticleAPIEnabled:                envBool("CCH_ENABLE_HELP_ARTICLE_API"),
		AnnouncementAPIEnabled:               envBool("CCH_ENABLE_ANNOUNCEMENT_API"),
		GuestIntakeSession: GuestIntakeSessionSettings{
			Enabled:        envBool("CCH_ENABLE_GUEST_INTAKE_SESSION_API"),
			SigningKey:     os.Getenv("CCH_INTAKE_SESSION_SECRET"),
			Issuer:         os.Getenv("CCH_INTAKE_SESSION_ISSUER"),
			TokenLifetime:  intakeLifetime,
			AllowedOrigins: splitEnvironmentList(os.Getenv("CCH_INTAKE_ALLOWED_ORIGINS")),
		},
		GuestIntakeSubmission: GuestIntakeSubmissionSettings{
			Enabled:          envBool("CCH_ENABLE_GUEST_INTAKE_SUBMISSION_API"),
			CCHBaseURL:       os.Getenv("CCH_INTAKE_CCH_API_BASE_URL"),
			CCHUsername:      os.Getenv("CCH_INTAKE_CCH_USERNAME"),
			CCHPassword:      os.Getenv("CCH_INTAKE_CCH_PASSWORD"),
			OriginOfficeCode: os.Getenv("CCH_INTAKE_OFFICE_CODE"),
		},
		MySQL: MySQLSettings{
			Host:     os.Getenv("CCH_DB_HOST"),
			Port:     port,
			User:     os.Getenv("CCH_DB_USER"),
			Password: os.Getenv("CCH_DB_PASSWORD"),
			Database: os.Getenv("CCH_DB_NAME"),
		},
		JWT: JWTSettings{
			Secret: os.Getenv("CCH_JWT_SECRET"),
			Issuer: os.Getenv("CCH_JWT_ISSUER"),
		},
	}
}

func splitEnvironmentList(value string) []string {
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func envBool(name string) bool {
	value, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && value
}
