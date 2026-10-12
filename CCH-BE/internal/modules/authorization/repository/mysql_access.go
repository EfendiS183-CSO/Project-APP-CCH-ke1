package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

const legacyProjectID = "{4FB290EB-B8DA-4CB0-919A-D9EEAA260785}"
const accountLookupQuery = "SELECT Username, User_Level, Report_To, Office, Activated, Locked FROM cch_users WHERE Username = ?"

var (
	ErrAccountNotFound                = authorization.ErrAccountNotFound
	ErrAccountInactive                = authorization.ErrAccountInactive
	ErrAccountLocked                  = authorization.ErrAccountLocked
	ErrIdentityMismatch               = authorization.ErrIdentityMismatch
	ErrSystemAdministratorUnsupported = authorization.ErrSystemAdministratorUnsupported
	ErrInvalidUserLevels              = authorization.ErrInvalidUserLevels
	ErrInvalidTableVariable           = authorization.ErrInvalidTableVariable
)

var tableVariablePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type AccountAccess = authorization.AccountAccess

type MySQLAccessRepository struct {
	db *sql.DB
}

var _ authorization.AccountAccessLoader = (*MySQLAccessRepository)(nil)

func NewMySQLAccessRepository(db *sql.DB) *MySQLAccessRepository {
	return &MySQLAccessRepository{db: db}
}

func (repository *MySQLAccessRepository) Load(ctx context.Context, identity authentication.VerifiedIdentity, tableVariable string) (AccountAccess, error) {
	if repository == nil || repository.db == nil {
		return AccountAccess{}, errors.New("authorization database is not configured")
	}
	if err := validateTableVariable(tableVariable); err != nil {
		return AccountAccess{}, ErrInvalidTableVariable
	}

	account, err := repository.loadAccount(ctx, identity.Username)
	if err != nil {
		return AccountAccess{}, err
	}
	if err := validateIdentity(identity, account); err != nil {
		return AccountAccess{}, err
	}
	account.PermissionMask, err = repository.loadPermissionMask(ctx, tableVariable, account.UserLevelIDs, identity.Permission)
	if err != nil {
		return AccountAccess{}, err
	}
	return account, nil
}

func (repository *MySQLAccessRepository) loadAccount(ctx context.Context, username string) (AccountAccess, error) {
	var account AccountAccess
	var currentLevels string
	var officeCode sql.NullString
	var reportTo sql.NullString
	var activated string
	var locked sql.NullString
	err := repository.db.QueryRowContext(ctx, accountLookupQuery, username).
		Scan(&account.Username, &currentLevels, &reportTo, &officeCode, &activated, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountAccess{}, ErrAccountNotFound
	}
	if err != nil {
		return AccountAccess{}, fmt.Errorf("load current CCH account: %w", err)
	}
	if !strings.EqualFold(activated, "Y") {
		return AccountAccess{}, ErrAccountInactive
	}
	if locked.Valid && strings.EqualFold(locked.String, "Y") {
		return AccountAccess{}, ErrAccountLocked
	}

	account.OfficeCode = nullableString(officeCode)
	account.ReportTo = nullableString(reportTo)
	account.UserLevelIDs, err = parseUserLevels(currentLevels)
	if err != nil {
		return AccountAccess{}, err
	}
	return account, nil
}

func validateIdentity(identity authentication.VerifiedIdentity, account AccountAccess) error {
	claimLevels, err := parseUserLevels(identity.UserLevelID)
	if err != nil {
		return ErrIdentityMismatch
	}
	if hasSystemAdministrator(account.UserLevelIDs) || hasSystemAdministrator(claimLevels) {
		return ErrSystemAdministratorUnsupported
	}
	if !sameUserLevels(account.UserLevelIDs, claimLevels) ||
		account.OfficeCode == "" || account.OfficeCode != identity.OfficeCode ||
		account.ReportTo != identity.ReportTo {
		return ErrIdentityMismatch
	}
	for _, levelID := range account.UserLevelIDs {
		if levelID < 0 {
			return ErrInvalidUserLevels
		}
	}
	return nil
}

func (repository *MySQLAccessRepository) loadPermissionMask(ctx context.Context, tableVariable string, userLevelIDs []int64, tokenMask int64) (int64, error) {
	permissionQuery := "SELECT Permission FROM cch_userlevelpermissions WHERE Table_Name = ? AND User_Level_ID IN (" + placeholders(len(userLevelIDs)) + ")"
	args := make([]any, 0, len(userLevelIDs)+1)
	args = append(args, legacyProjectID+tableVariable)
	for _, levelID := range userLevelIDs {
		args = append(args, levelID)
	}
	rows, err := repository.db.QueryContext(ctx, permissionQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("load CCH table permissions: %w", err)
	}
	defer rows.Close()

	roleMasks := make([]int64, 0, len(userLevelIDs))
	for rows.Next() {
		var mask int64
		if err := rows.Scan(&mask); err != nil {
			return 0, fmt.Errorf("scan CCH table permission: %w", err)
		}
		roleMasks = append(roleMasks, mask)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read CCH table permissions: %w", err)
	}
	return authorization.EffectiveMask(roleMasks, false, tokenMask), nil
}

func validateTableVariable(tableVariable string) error {
	if !tableVariablePattern.MatchString(tableVariable) || len(legacyProjectID)+len(tableVariable) > 191 {
		return ErrInvalidTableVariable
	}
	return nil
}

func parseUserLevels(value string) ([]int64, error) {
	parts := strings.Split(value, ",")
	levels := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, ErrInvalidUserLevels
		}
		var levelID int64
		if _, err := fmt.Sscan(part, &levelID); err != nil || fmt.Sprint(levelID) != part {
			return nil, ErrInvalidUserLevels
		}
		if _, exists := seen[levelID]; !exists {
			levels = append(levels, levelID)
			seen[levelID] = struct{}{}
		}
	}
	if len(levels) == 0 {
		return nil, ErrInvalidUserLevels
	}
	return levels, nil
}

func sameUserLevels(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for _, levelID := range left {
		found := false
		for _, otherID := range right {
			if levelID == otherID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func hasSystemAdministrator(levels []int64) bool {
	for _, levelID := range levels {
		if levelID == -1 {
			return true
		}
	}
	return false
}

func placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
