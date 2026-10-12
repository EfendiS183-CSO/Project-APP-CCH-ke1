package authorization

import (
	"context"
	"errors"
	"testing"

	"cch.local/backend/internal/platform/authentication"
)

type stubAccountAccessLoader struct {
	access          AccountAccess
	err             error
	called          bool
	loadedIdentity  authentication.VerifiedIdentity
	loadedTableName string
}

func (loader *stubAccountAccessLoader) Load(_ context.Context, identity authentication.VerifiedIdentity, tableVariable string) (AccountAccess, error) {
	loader.called = true
	loader.loadedIdentity = identity
	loader.loadedTableName = tableVariable
	return loader.access, loader.err
}

func TestAccessServiceAuthorizesAllowedActionAndReturnsTrustedScope(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	loader := &stubAccountAccessLoader{
		access: AccountAccess{
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			ReportTo:       "40005",
			PermissionMask: int64(ActionList | ActionView),
		},
	}

	access, err := NewAccessService(loader).Authorize(context.Background(), identity, "cchentri", ActionList)
	if err != nil {
		t.Fatal(err)
	}
	if !loader.called || loader.loadedIdentity != identity || loader.loadedTableName != "cchentri" {
		t.Fatal("access service did not load authorization for the requested identity and table")
	}
	if access.OfficeCode != "40005CC" || access.ReportTo != "40005" {
		t.Fatalf("trusted office scope = %q/%q, want 40005CC/40005", access.OfficeCode, access.ReportTo)
	}
}

func TestAccessServiceRequiresEveryRequestedAction(t *testing.T) {
	loader := &stubAccountAccessLoader{access: AccountAccess{PermissionMask: int64(ActionList)}}
	_, err := NewAccessService(loader).Authorize(context.Background(), authentication.VerifiedIdentity{}, "cchentri", ActionList|ActionView)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize() error = %v, want permission denied", err)
	}
}

func TestAccessServiceRejectsInvalidActionsBeforeLoading(t *testing.T) {
	tests := []struct {
		name   string
		action Action
	}{
		{name: "zero", action: 0},
		{name: "negative", action: -1},
		{name: "unknown bit", action: Action(1 << 20)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loader := &stubAccountAccessLoader{}
			_, err := NewAccessService(loader).Authorize(context.Background(), authentication.VerifiedIdentity{}, "cchentri", test.action)
			if !errors.Is(err, ErrInvalidAction) {
				t.Fatalf("Authorize() error = %v, want invalid action", err)
			}
			if loader.called {
				t.Fatal("invalid action reached the account access loader")
			}
		})
	}
}

func TestAccessServiceRejectsCorruptPermissionMasks(t *testing.T) {
	for _, mask := range []int64{-1, AllPermissions + 1} {
		loader := &stubAccountAccessLoader{access: AccountAccess{PermissionMask: mask}}
		_, err := NewAccessService(loader).Authorize(context.Background(), authentication.VerifiedIdentity{}, "cchentri", ActionView)
		if !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("Authorize() with mask %d error = %v, want permission denied", mask, err)
		}
	}
}

func TestAccessServicePropagatesLoaderError(t *testing.T) {
	wantErr := errors.New("account lookup failed")
	loader := &stubAccountAccessLoader{err: wantErr}
	_, err := NewAccessService(loader).Authorize(context.Background(), authentication.VerifiedIdentity{}, "cchentri", ActionView)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Authorize() error = %v, want loader error", err)
	}
}

func TestAccessServiceRequiresLoader(t *testing.T) {
	var service *AccessService
	_, err := service.Authorize(context.Background(), authentication.VerifiedIdentity{}, "cchentri", ActionView)
	if !errors.Is(err, ErrAccessServiceNotConfigured) {
		t.Fatalf("Authorize() error = %v, want service not configured", err)
	}
}
