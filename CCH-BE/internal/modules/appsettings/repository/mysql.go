package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cch.local/backend/internal/modules/appsettings"
)

const listSettingsSQL = `SELECT Option_ID, Option_Default, Show_Announcement, Use_Announcement_Table,
Maintenance_Mode, Maintenance_Finish_DateTime, Auto_Normal_After_Maintenance
FROM cch_settings ORDER BY Option_ID ASC`
const getSettingsSQL = `SELECT Option_ID, Option_Default, Show_Announcement, Use_Announcement_Table,
Maintenance_Mode, Maintenance_Finish_DateTime, Auto_Normal_After_Maintenance
FROM cch_settings WHERE Option_ID = ? LIMIT 1`
const insertSettingsSQL = `INSERT INTO cch_settings
(Option_Default, Show_Announcement, Use_Announcement_Table, Maintenance_Mode,
Maintenance_Finish_DateTime, Auto_Normal_After_Maintenance)
VALUES (?, ?, ?, ?, ?, ?)`
const updateSettingsSQL = `UPDATE cch_settings SET
Show_Announcement = ?, Use_Announcement_Table = ?, Maintenance_Mode = ?,
Maintenance_Finish_DateTime = ?, Auto_Normal_After_Maintenance = ?
WHERE Option_ID = ?`
const deleteSettingsSQL = `DELETE FROM cch_settings WHERE Option_ID = ?`
const otherDefaultSettingsSQL = `SELECT Option_ID FROM cch_settings WHERE Option_Default = 'Y' AND Option_ID <> ? LIMIT 1`
const defaultSettingsExistsSQL = `SELECT Option_ID FROM cch_settings WHERE Option_ID = ? AND Option_Default = 'Y' LIMIT 1`
const settingsExistsSQL = `SELECT Option_ID FROM cch_settings WHERE Option_ID = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("app settings repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ appsettings.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]appsettings.Settings, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listSettingsSQL)
	if err != nil {
		return nil, fmt.Errorf("list app settings: %w", err)
	}
	defer rows.Close()
	values := make([]appsettings.Settings, 0)
	for rows.Next() {
		value, err := scanSettings(rows)
		if err != nil {
			return nil, fmt.Errorf("scan app settings: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read app settings: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (appsettings.Settings, error) {
	if repository == nil || repository.db == nil {
		return appsettings.Settings{}, ErrRepositoryNotConfigured
	}
	value, err := scanSettings(repository.db.QueryRowContext(ctx, getSettingsSQL, id))
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("get app settings: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value appsettings.Settings) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if err := repository.checkOtherDefault(ctx, value.OptionID, value.IsDefaultOption); err != nil {
		return err
	}
	if _, err := repository.db.ExecContext(ctx, insertSettingsSQL,
		boolToEnum(value.IsDefaultOption), boolToEnum(value.ShowAnnouncement),
		boolToEnum(value.UseAnnouncementTable), boolToEnum(value.MaintenanceMode),
		value.MaintenanceFinishDateTime, boolToEnum(value.AutoNormalAfterMaintenance)); err != nil {
		return fmt.Errorf("insert app settings: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value appsettings.Settings) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateSettingsSQL,
		boolToEnum(value.ShowAnnouncement), boolToEnum(value.UseAnnouncementTable),
		boolToEnum(value.MaintenanceMode), value.MaintenanceFinishDateTime,
		boolToEnum(value.AutoNormalAfterMaintenance), value.OptionID)
	if err != nil {
		return fmt.Errorf("update app settings: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check app settings update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.OptionID)
}

func (repository *MySQLRepository) Delete(ctx context.Context, id int64) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deleteSettingsSQL, id)
	if err != nil {
		return fmt.Errorf("delete app settings: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check app settings delete: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, id)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, id int64) error {
	var existing int64
	err := repository.db.QueryRowContext(ctx, settingsExistsSQL, id).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return appsettings.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check app settings: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) checkOtherDefault(ctx context.Context, id int64, isDefault bool) error {
	if !isDefault {
		return nil
	}
	var other int64
	err := repository.db.QueryRowContext(ctx, otherDefaultSettingsSQL, id).Scan(&other)
	if err == nil {
		return appsettings.ErrOnlyOneDefault
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check app settings default: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanSettings(row rowScanner) (appsettings.Settings, error) {
	var value appsettings.Settings
	var isDefault, showAnnouncement, useAnnouncementTable, maintenanceMode, autoNormal string
	var maintenanceFinish sql.NullTime
	if err := row.Scan(
		&value.OptionID, &isDefault, &showAnnouncement, &useAnnouncementTable, &maintenanceMode,
		&maintenanceFinish, &autoNormal); err != nil {
		return appsettings.Settings{}, err
	}
	value.IsDefaultOption = isDefault == "Y"
	value.ShowAnnouncement = showAnnouncement == "Y"
	value.UseAnnouncementTable = useAnnouncementTable == "Y"
	value.MaintenanceMode = maintenanceMode == "Y"
	value.AutoNormalAfterMaintenance = autoNormal == "Y"
	if maintenanceFinish.Valid {
		normalized := maintenanceFinish.Time.UTC().Truncate(time.Second)
		value.MaintenanceFinishDateTime = &normalized
	}
	return value, nil
}

func boolToEnum(enabled bool) string {
	if enabled {
		return "Y"
	}
	return "N"
}
