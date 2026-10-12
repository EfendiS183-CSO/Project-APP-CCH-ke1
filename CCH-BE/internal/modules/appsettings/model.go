package appsettings

import (
	"context"
	"errors"
	"time"
)

const TableVariable = "cch_settings"

var (
	ErrInvalidInput            = errors.New("invalid app settings input")
	ErrNotFound                = errors.New("app settings row not found")
	ErrDeleteRestricted        = errors.New("app settings cannot be deleted")
	ErrOnlyOneDefault          = errors.New("only one app settings row can be the default")
	ErrCannotSetMaintenanceNow = errors.New("maintenance finish datetime is required when maintenance mode is on")
	ErrServiceNotReady         = errors.New("app settings service is not configured")
)

// Settings mirrors one legacy cch_settings row. The legacy runtime reads the
// row with Option_Default='Y' through LoadApplicationSettings() to drive
// MS_MAINTENANCE_MODE / announcements, so the default row must never be deleted
// and maintenance fields are validated together.
type Settings struct {
	OptionID                   int64      `json:"option_id"`
	IsDefaultOption            bool       `json:"is_default_option"`
	ShowAnnouncement           bool       `json:"show_announcement"`
	UseAnnouncementTable       bool       `json:"use_announcement_table"`
	MaintenanceMode            bool       `json:"maintenance_mode"`
	MaintenanceFinishDateTime  *time.Time `json:"maintenance_finish_datetime,omitempty"`
	AutoNormalAfterMaintenance bool       `json:"auto_normal_after_maintenance"`
}

type CreateCommand struct {
	IsDefaultOption            bool
	ShowAnnouncement           bool
	UseAnnouncementTable       bool
	MaintenanceMode            bool
	MaintenanceFinishDateTime  *time.Time
	AutoNormalAfterMaintenance bool
}

type UpdateCommand struct {
	OptionID                   int64
	ShowAnnouncement           bool
	UseAnnouncementTable       bool
	MaintenanceMode            bool
	MaintenanceFinishDateTime  *time.Time
	AutoNormalAfterMaintenance bool
}

type Repository interface {
	List(context.Context) ([]Settings, error)
	Get(context.Context, int64) (Settings, error)
	Insert(context.Context, Settings) error
	Update(context.Context, Settings) error
	Delete(context.Context, int64) error
}

func validateCreate(command CreateCommand) (Settings, error) {
	value := Settings{
		IsDefaultOption:            command.IsDefaultOption,
		ShowAnnouncement:           command.ShowAnnouncement,
		UseAnnouncementTable:       command.UseAnnouncementTable,
		MaintenanceMode:            command.MaintenanceMode,
		MaintenanceFinishDateTime:  normalizeTime(command.MaintenanceFinishDateTime),
		AutoNormalAfterMaintenance: command.AutoNormalAfterMaintenance,
	}
	return value, value.validate()
}

func validateUpdate(command UpdateCommand) (Settings, error) {
	value := Settings{
		OptionID:                   command.OptionID,
		ShowAnnouncement:           command.ShowAnnouncement,
		UseAnnouncementTable:       command.UseAnnouncementTable,
		MaintenanceMode:            command.MaintenanceMode,
		MaintenanceFinishDateTime:  normalizeTime(command.MaintenanceFinishDateTime),
		AutoNormalAfterMaintenance: command.AutoNormalAfterMaintenance,
	}
	if value.OptionID < 1 {
		return Settings{}, ErrInvalidInput
	}
	return value, value.validate()
}

func (value Settings) validate() error {
	if value.MaintenanceMode && value.MaintenanceFinishDateTime == nil {
		return ErrCannotSetMaintenanceNow
	}
	return nil
}

func normalizeTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	// Strip monotonic + timezone offset so the stored DATETIME is unambiguous.
	normalized := value.UTC().Truncate(time.Second)
	return &normalized
}

func validateID(id int64) error {
	if id < 1 {
		return ErrInvalidInput
	}
	return nil
}
