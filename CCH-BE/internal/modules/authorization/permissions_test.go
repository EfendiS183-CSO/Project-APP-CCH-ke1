package authorization

import "testing"

func TestEffectiveMaskCombinesRolePermissions(t *testing.T) {
	mask := EffectiveMask([]int64{int64(ActionList | ActionView), int64(ActionAdd)}, false, 0)
	if !Allows(mask, ActionList) || !Allows(mask, ActionView) || !Allows(mask, ActionAdd) {
		t.Fatalf("combined mask %b is missing a role permission", mask)
	}
	if Allows(mask, ActionDelete) {
		t.Fatalf("combined mask %b unexpectedly allows delete", mask)
	}
}

func TestTokenMaskRestrictsCombinedRolePermissions(t *testing.T) {
	roleMask := int64(ActionList | ActionView | ActionAdd)
	mask := EffectiveMask([]int64{roleMask}, false, int64(ActionList|ActionView))
	if !Allows(mask, ActionList) || !Allows(mask, ActionView) {
		t.Fatalf("effective mask %b lost allowed token permissions", mask)
	}
	if Allows(mask, ActionAdd) {
		t.Fatalf("effective mask %b did not apply token restriction", mask)
	}
}

func TestZeroTokenMaskDoesNotRestrictRolePermissions(t *testing.T) {
	roleMask := int64(ActionList | ActionAdd)
	if got := EffectiveMask([]int64{roleMask}, false, 0); got != roleMask {
		t.Fatalf("effective mask = %b, want %b", got, roleMask)
	}
}

func TestSystemAdministratorMaskIsStillLimitedByTokenMask(t *testing.T) {
	if got := EffectiveMask(nil, true, 0); got != AllPermissions {
		t.Fatalf("admin mask = %b, want %b", got, AllPermissions)
	}
	mask := EffectiveMask(nil, true, int64(ActionView))
	if !Allows(mask, ActionView) || Allows(mask, ActionDelete) {
		t.Fatalf("admin token mask = %b, want view-only", mask)
	}
}
