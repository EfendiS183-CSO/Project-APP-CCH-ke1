package followups

import (
	"context"
	"errors"
	"testing"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type stubPlanRepository struct {
	command CreatePlanCommand
	entry   Entry
	err     error
	called  bool
}

type stubRealizationRepository struct {
	command RealizeCommand
	result  RealizeResult
	err     error
	called  bool
}

func (repository *stubRealizationRepository) RealizeCallCenterPlan(_ context.Context, command RealizeCommand) (RealizeResult, error) {
	repository.called = true
	repository.command = command
	return repository.result, repository.err
}

func (repository *stubPlanRepository) CreateCallCenterPlan(_ context.Context, command CreatePlanCommand) (Entry, error) {
	repository.called = true
	repository.command = command
	return repository.entry, repository.err
}

func TestCallCenterPlanServiceUsesTrustedOfficeAndNormalizesRequest(t *testing.T) {
	access := authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		ReportTo:       "40005",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionAdd),
	}
	loader := &stubAccessLoader{
		byTable: map[string]authorization.AccountAccess{
			complaintTableName: access,
			followUpTableName:  access,
		},
	}
	repository := &stubPlanRepository{entry: Entry{ComplaintID: "TKT-001", OfficeCode: "40005CC", StatusCode: "FU1"}}
	service := NewCallCenterPlanService(authorization.NewAccessService(loader), repository)
	plannedAt := time.Date(2026, time.October, 1, 10, 30, 45, 900_000_000, time.FixedZone("test", 7*60*60))
	entry, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}, CreatePlanRequest{
		ComplaintID: " TKT-001 ", StatusCode: " FU1 ", PlannedAt: plannedAt, PlanNotes: "  Hubungi pelanggan  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || entry.OfficeCode != "40005CC" {
		t.Fatalf("repository/entry = %#v/%#v", repository, entry)
	}
	command := repository.command
	if command.ComplaintID != "TKT-001" || command.StatusCode != "FU1" || command.OfficeCode != "40005CC" || command.Actor != "agent.test" || command.PlanNotes != "Hubungi pelanggan" {
		t.Fatalf("plan command = %#v", command)
	}
	if command.PlannedAt.Location() != time.UTC || command.PlannedAt.Nanosecond() != 0 {
		t.Fatalf("planned timestamp was not normalized: %v", command.PlannedAt)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != complaintTableName || loader.loadedTables[1] != followUpTableName {
		t.Fatalf("authorization tables = %#v", loader.loadedTables)
	}
}

func TestCallCenterPlanServiceRejectsInvalidInputBeforeAuthorization(t *testing.T) {
	loader := &stubAccessLoader{}
	repository := &stubPlanRepository{}
	service := NewCallCenterPlanService(authorization.NewAccessService(loader), repository)
	_, err := service.Create(context.Background(), authentication.VerifiedIdentity{}, CreatePlanRequest{
		ComplaintID: "TKT-001", StatusCode: "FU1", PlanNotes: "plan",
	})
	if !errors.Is(err, ErrInvalidPlanRequest) {
		t.Fatalf("Create() error = %v, want invalid request", err)
	}
	if repository.called || len(loader.loadedTables) != 0 {
		t.Fatal("invalid request reached authorization or repository")
	}
}

func TestCallCenterPlanServiceRequiresAddViewSingleRoleAndStableScope(t *testing.T) {
	for _, test := range []struct {
		name      string
		complaint authorization.AccountAccess
		followUp  authorization.AccountAccess
		wantErr   error
	}{
		{
			name:      "missing Add permission",
			complaint: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView)},
			followUp:  authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView)},
			wantErr:   authorization.ErrPermissionDenied,
		},
		{
			name:      "wrong role",
			complaint: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionView)},
			followUp:  authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionAdd)},
			wantErr:   ErrCallCenterRoleRequired,
		},
		{
			name:      "office mismatch",
			complaint: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView)},
			followUp:  authorization.AccountAccess{OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionAdd)},
			wantErr:   authorization.ErrIdentityMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &stubAccessLoader{byTable: map[string]authorization.AccountAccess{
				complaintTableName: test.complaint,
				followUpTableName:  test.followUp,
			}}
			repository := &stubPlanRepository{}
			service := NewCallCenterPlanService(authorization.NewAccessService(loader), repository)
			_, err := service.Create(context.Background(), authentication.VerifiedIdentity{}, CreatePlanRequest{
				ComplaintID: "TKT-001", StatusCode: "FU1", PlannedAt: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, test.wantErr)
			}
			if repository.called {
				t.Fatal("unauthorized plan reached repository")
			}
		})
	}
}

func TestCallCenterRealizationServiceUsesTrustedOfficeAndActor(t *testing.T) {
	access := authorization.AccountAccess{
		Username: "agent.test", OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionEdit),
	}
	loader := &stubAccessLoader{byTable: map[string]authorization.AccountAccess{followUpTableName: access}}
	repository := &stubRealizationRepository{result: RealizeResult{Entry: Entry{ComplaintID: "TKT-001", OfficeCode: "40005CC", StatusCode: "FU1"}}}
	service := NewCallCenterRealizationService(authorization.NewAccessService(loader), repository)
	result, err := service.Realize(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, RealizeRequest{
		ComplaintID: " TKT-001 ", StatusCode: " FU1 ", Outcome: "  Pelanggan telah dihubungi  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || result.Entry.ComplaintID != "TKT-001" || repository.command.OfficeCode != "40005CC" || repository.command.Actor != "agent.test" || repository.command.Outcome != "Pelanggan telah dihubungi" {
		t.Fatalf("result/command = %#v/%#v", result, repository.command)
	}
	if len(loader.loadedTables) != 1 || loader.loadedTables[0] != followUpTableName {
		t.Fatalf("authorization tables = %#v", loader.loadedTables)
	}
}

func TestCallCenterRealizationServiceRejectsInvalidAndUnauthorizedRequests(t *testing.T) {
	loader := &stubAccessLoader{}
	repository := &stubRealizationRepository{}
	service := NewCallCenterRealizationService(authorization.NewAccessService(loader), repository)
	_, err := service.Realize(context.Background(), authentication.VerifiedIdentity{}, RealizeRequest{ComplaintID: "TKT-001", StatusCode: "FU1"})
	if !errors.Is(err, ErrInvalidRealization) || repository.called || len(loader.loadedTables) != 0 {
		t.Fatalf("invalid realization error/calls = %v/%v/%#v", err, repository.called, loader.loadedTables)
	}
	loader = &stubAccessLoader{defaultAccess: authorization.AccountAccess{
		OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionList),
	}}
	repository = &stubRealizationRepository{}
	service = NewCallCenterRealizationService(authorization.NewAccessService(loader), repository)
	_, err = service.Realize(context.Background(), authentication.VerifiedIdentity{}, RealizeRequest{
		ComplaintID: "TKT-001", StatusCode: "FU1", Outcome: "Contacted",
	})
	if !errors.Is(err, authorization.ErrPermissionDenied) || repository.called {
		t.Fatalf("unauthorized realization error/called = %v/%v", err, repository.called)
	}
}
