package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/announcements"
)

const listAnnouncementsQuery = `SELECT Announcement_ID, Is_Active, Topic, Message, Date_LastUpdate, Language,
Auto_Publish, Date_Start, Date_End, Date_Created, Created_By, Translated_ID
FROM cch_announcement ORDER BY Date_LastUpdate DESC, Announcement_ID DESC LIMIT ? OFFSET ?`

const countAnnouncementsQuery = `SELECT COUNT(*) FROM cch_announcement WHERE Language = ?`

const languageOptionsQuery = `SELECT Language_Code, Language_Name FROM cch_languages ORDER BY Language_Code ASC`

const languageExistsQuery = `SELECT 1 FROM cch_languages WHERE Language_Code = ? LIMIT 1`

const getAnnouncementQuery = `SELECT Announcement_ID, Is_Active, Topic, Message, Date_LastUpdate, Language,
Auto_Publish, Date_Start, Date_End, Date_Created, Created_By, Translated_ID
FROM cch_announcement WHERE Announcement_ID = ? LIMIT 1`

const insertionOverlapQuery = `SELECT Announcement_ID, Date_Start, Date_End FROM cch_announcement
WHERE Language = ? AND Date_Start IS NOT NULL AND Date_End IS NOT NULL AND Announcement_ID <> ?
AND Date_Start <= ? AND Date_End >= ? LIMIT 1`

const languageTranslationCountQuery = `SELECT COUNT(*) FROM cch_announcement WHERE Language = ? AND Translated_ID = ?`

const insertAnnouncementQuery = `INSERT INTO cch_announcement
(Is_Active, Topic, Message, Language, Auto_Publish, Date_Start, Date_End, Date_Created, Created_By, Translated_ID)
VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), ?, ?)`

const lockAnnouncementQuery = `SELECT Announcement_ID FROM cch_announcement WHERE Announcement_ID = ? FOR UPDATE`

const updateAnnouncementQuery = `UPDATE cch_announcement SET Is_Active = ?, Topic = ?, Message = ?, Language = ?,
Auto_Publish = ?, Date_Start = ?, Date_End = ?, Date_LastUpdate = NOW(), Translated_ID = ?
WHERE Announcement_ID = ?`

const deleteChildReferencesQuery = `SELECT Announcement_ID FROM cch_announcement WHERE Translated_ID = ? LIMIT 1`

const deleteAnnouncementQuery = `DELETE FROM cch_announcement WHERE Announcement_ID = ?`

var ErrRepositoryNotConfigured = errors.New("announcement repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ announcements.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: database}
}

func (repository *MySQLRepository) List(ctx context.Context, query announcements.ListQuery) (announcements.Page, error) {
	if repository == nil || repository.db == nil {
		return announcements.Page{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return announcements.Page{}, err
	}
	var total int64
	if err := repository.db.QueryRowContext(ctx, countAnnouncementsQuery, query.Language).Scan(&total); err != nil {
		return announcements.Page{}, fmt.Errorf("count announcements: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, listAnnouncementsQuery, query.Limit, query.Offset)
	if err != nil {
		return announcements.Page{}, fmt.Errorf("list announcements: %w", err)
	}
	defer rows.Close()
	values := make([]announcements.Announcement, 0, query.Limit)
	for rows.Next() {
		value, err := scanAnnouncement(rows)
		if err != nil {
			return announcements.Page{}, fmt.Errorf("scan announcement: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return announcements.Page{}, fmt.Errorf("read announcements: %w", err)
	}
	return announcements.Page{Data: values, Total: total, Limit: query.Limit, Offset: query.Offset}, nil
}

func (repository *MySQLRepository) Options(ctx context.Context) (announcements.Options, error) {
	if repository == nil || repository.db == nil {
		return announcements.Options{}, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, languageOptionsQuery)
	if err != nil {
		return announcements.Options{}, fmt.Errorf("list announcement languages: %w", err)
	}
	defer rows.Close()
	options := announcements.Options{Languages: make([]announcements.LanguageOption, 0)}
	for rows.Next() {
		var option announcements.LanguageOption
		if err := rows.Scan(&option.Code, &option.Name); err != nil {
			return announcements.Options{}, fmt.Errorf("scan announcement language: %w", err)
		}
		options.Languages = append(options.Languages, option)
	}
	if err := rows.Err(); err != nil {
		return announcements.Options{}, fmt.Errorf("read announcement languages: %w", err)
	}
	return options, nil
}

func (repository *MySQLRepository) LanguageExists(ctx context.Context, language string) (bool, error) {
	if repository == nil || repository.db == nil {
		return false, ErrRepositoryNotConfigured
	}
	var marker int
	err := repository.db.QueryRowContext(ctx, languageExistsQuery, language).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check announcement language: %w", err)
	}
	return true, nil
}

func (repository *MySQLRepository) CountByLanguage(ctx context.Context, language string, translatedID int64) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, ErrRepositoryNotConfigured
	}
	var count int64
	if err := repository.db.QueryRowContext(ctx, languageTranslationCountQuery, language, translatedID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count announcement translation: %w", err)
	}
	return count, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (announcements.Announcement, error) {
	if repository == nil || repository.db == nil {
		return announcements.Announcement{}, ErrRepositoryNotConfigured
	}
	value, err := scanAnnouncement(repository.db.QueryRowContext(ctx, getAnnouncementQuery, id))
	if err != nil {
		return announcements.Announcement{}, fmt.Errorf("get announcement: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) FindOverlap(ctx context.Context, language string, start, end time.Time, excludeID int64) (bool, error) {
	if repository == nil || repository.db == nil {
		return false, ErrRepositoryNotConfigured
	}
	var marker int64
	err := repository.db.QueryRowContext(ctx, insertionOverlapQuery, language, excludeID, end, start).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check announcement overlap: %w", err)
	}
	return true, nil
}

func (repository *MySQLRepository) Create(ctx context.Context, value announcements.Announcement, actor string) (announcements.Announcement, error) {
	if repository == nil || repository.db == nil {
		return announcements.Announcement{}, ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, insertAnnouncementQuery,
		yn(value.IsActive), value.Topic, value.Message, value.Language, yn(value.AutoPublish),
		nullableTime(value.DateStart), nullableTime(value.DateEnd), nullableActor(actor), nullableID(value.TranslatedID))
	if err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return announcements.Announcement{}, announcements.ErrDuplicateTranslation
		}
		return announcements.Announcement{}, fmt.Errorf("insert announcement: %w", err)
	}
	value.ID, err = result.LastInsertId()
	if err != nil {
		return announcements.Announcement{}, fmt.Errorf("read new announcement ID: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value announcements.Announcement, actor string) (announcements.Announcement, error) {
	if repository == nil || repository.db == nil {
		return announcements.Announcement{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return announcements.Announcement{}, fmt.Errorf("begin announcement update: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, lockAnnouncementQuery, value.ID).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return announcements.Announcement{}, announcements.ErrNotFound
		}
		return announcements.Announcement{}, fmt.Errorf("lock announcement: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, updateAnnouncementQuery,
		yn(value.IsActive), value.Topic, value.Message, value.Language, yn(value.AutoPublish),
		nullableTime(value.DateStart), nullableTime(value.DateEnd), nullableID(value.TranslatedID), value.ID); err != nil {
		return announcements.Announcement{}, fmt.Errorf("update announcement: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return announcements.Announcement{}, fmt.Errorf("commit announcement update: %w", err)
	}
	value.DateLastUpdate = time.Now()
	return value, nil
}

func (repository *MySQLRepository) Delete(ctx context.Context, id int64) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin announcement delete: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, lockAnnouncementQuery, id).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return announcements.ErrNotFound
		}
		return fmt.Errorf("lock announcement for delete: %w", err)
	}
	var childID int64
	if err := transaction.QueryRowContext(ctx, deleteChildReferencesQuery, id).Scan(&childID); err == nil {
		return announcements.ErrInUse
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check announcement child references: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, deleteAnnouncementQuery, id); err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit announcement delete: %w", err)
	}
	return nil
}

type announcementRowScanner interface{ Scan(...any) error }

func scanAnnouncement(row announcementRowScanner) (announcements.Announcement, error) {
	var value announcements.Announcement
	var active, autoPublish string
	var dateStart, dateEnd, dateCreated sql.NullTime
	var createdBy sql.NullString
	var translatedID sql.NullInt64
	if err := row.Scan(&value.ID, &active, &value.Topic, &value.Message, &value.DateLastUpdate, &value.Language,
		&autoPublish, &dateStart, &dateEnd, &dateCreated, &createdBy, &translatedID); err != nil {
		return announcements.Announcement{}, err
	}
	value.IsActive = active == "Y"
	value.AutoPublish = autoPublish == "Y"
	if dateStart.Valid {
		value.DateStart = &dateStart.Time
	}
	if dateEnd.Valid {
		value.DateEnd = &dateEnd.Time
	}
	if dateCreated.Valid {
		value.DateCreated = &dateCreated.Time
	}
	if createdBy.Valid {
		value.CreatedBy = &createdBy.String
	}
	if translatedID.Valid {
		value.TranslatedID = &translatedID.Int64
	}
	return value, nil
}

func yn(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableID(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableActor(value string) any {
	if value == "" {
		return nil
	}
	return value
}
