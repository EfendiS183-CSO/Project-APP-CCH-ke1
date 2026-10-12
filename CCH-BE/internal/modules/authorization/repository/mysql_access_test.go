package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

func TestLoadCombinesCurrentRolePermissionsAndTokenMask(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	identity := testIdentity("3, 4")
	identity.Permission = int64(authorization.ActionList | authorization.ActionView)
	mock.ExpectQuery(regexp.QuoteMeta(accountLookupQuery)).
		WithArgs(identity.Username).
		WillReturnRows(sqlmock.NewRows([]string{"Username", "User_Level", "Report_To", "Office", "Activated", "Locked"}).
			AddRow(identity.Username, "3,4", "40005", "40005CC", "Y", "N"))
	aclQuery := "SELECT Permission FROM cch_userlevelpermissions WHERE Table_Name = ? AND User_Level_ID IN (?,?)"
	mock.ExpectQuery(regexp.QuoteMeta(aclQuery)).
		WithArgs(legacyProjectID+"cchentri", int64(3), int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"Permission"}).AddRow(int64(authorization.ActionList | authorization.ActionView | authorization.ActionAdd)).AddRow(int64(authorization.ActionDelete)))

	access, err := NewMySQLAccessRepository(database).Load(context.Background(), identity, "cchentri")
	if err != nil {
		t.Fatal(err)
	}
	if access.PermissionMask != int64(authorization.ActionList|authorization.ActionView) {
		t.Fatalf("permission mask = %b, want list and view", access.PermissionMask)
	}
	if access.OfficeCode != "40005CC" || access.ReportTo != "40005" {
		t.Fatalf("trusted office scope = %q/%q, want 40005CC/40005", access.OfficeCode, access.ReportTo)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsUnavailableOrStaleAccounts(t *testing.T) {
	tests := []struct {
		name      string
		activated string
		locked    string
		office    string
		reportTo  string
		levels    string
		wantErr   error
	}{
		{name: "inactive", activated: "N", locked: "N", office: "40005CC", reportTo: "40005", levels: "3", wantErr: ErrAccountInactive},
		{name: "locked", activated: "Y", locked: "Y", office: "40005CC", reportTo: "40005", levels: "3", wantErr: ErrAccountLocked},
		{name: "office changed", activated: "Y", locked: "N", office: "40006CC", reportTo: "40005", levels: "3", wantErr: ErrIdentityMismatch},
		{name: "report-to changed", activated: "Y", locked: "N", office: "40005CC", reportTo: "40006", levels: "3", wantErr: ErrIdentityMismatch},
		{name: "role changed", activated: "Y", locked: "N", office: "40005CC", reportTo: "40005", levels: "4", wantErr: ErrIdentityMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			identity := testIdentity("3")
			mock.ExpectQuery(regexp.QuoteMeta(accountLookupQuery)).
				WithArgs(identity.Username).
				WillReturnRows(sqlmock.NewRows([]string{"Username", "User_Level", "Report_To", "Office", "Activated", "Locked"}).
					AddRow(identity.Username, test.levels, test.reportTo, test.office, test.activated, test.locked))

			_, err = NewMySQLAccessRepository(database).Load(context.Background(), identity, "cchentri")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Load() error = %v, want %v", err, test.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoadDeniesSystemAdministratorClaimsAndAccount(t *testing.T) {
	for _, test := range []struct {
		name          string
		claimLevels   string
		accountLevels string
	}{
		{name: "token claim", claimLevels: "-1", accountLevels: "3"},
		{name: "current account", claimLevels: "3", accountLevels: "-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			identity := testIdentity(test.claimLevels)
			mock.ExpectQuery(regexp.QuoteMeta(accountLookupQuery)).
				WithArgs(identity.Username).
				WillReturnRows(sqlmock.NewRows([]string{"Username", "User_Level", "Report_To", "Office", "Activated", "Locked"}).
					AddRow(identity.Username, test.accountLevels, "40005", "40005CC", "Y", "N"))

			_, err = NewMySQLAccessRepository(database).Load(context.Background(), identity, "cchentri")
			if !errors.Is(err, ErrSystemAdministratorUnsupported) {
				t.Fatalf("Load() error = %v, want system administrator denial", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoadUsesExactProjectTableKeyAndUnknownTableHasNoPermissions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	identity := testIdentity("3")
	mock.ExpectQuery(regexp.QuoteMeta(accountLookupQuery)).
		WithArgs(identity.Username).
		WillReturnRows(sqlmock.NewRows([]string{"Username", "User_Level", "Report_To", "Office", "Activated", "Locked"}).
			AddRow(identity.Username, "3", "40005", "40005CC", "Y", "N"))
	aclQuery := "SELECT Permission FROM cch_userlevelpermissions WHERE Table_Name = ? AND User_Level_ID IN (?)"
	mock.ExpectQuery(regexp.QuoteMeta(aclQuery)).
		WithArgs(legacyProjectID+"unknown_table", int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"Permission"}))

	access, err := NewMySQLAccessRepository(database).Load(context.Background(), identity, "unknown_table")
	if err != nil {
		t.Fatal(err)
	}
	if access.PermissionMask != 0 {
		t.Fatalf("unknown table permission mask = %b, want 0", access.PermissionMask)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDoesNotSelectPassword(t *testing.T) {
	if regexp.MustCompile(`(?i)password`).MatchString(accountLookupQuery) {
		t.Fatal("account lookup must not select password data")
	}
}

func testIdentity(userLevels string) authentication.VerifiedIdentity {
	return authentication.VerifiedIdentity{
		Username:    "agent.test",
		OfficeCode:  "40005CC",
		ReportTo:    "40005",
		UserLevelID: userLevels,
		Permission:  524287,
	}
}
