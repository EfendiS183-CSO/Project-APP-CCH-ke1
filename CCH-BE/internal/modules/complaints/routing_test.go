package complaints

import "testing"

func TestComplaintRoutingInvolvesOfficeAcrossLegacyRoutingFields(t *testing.T) {
	routing := ComplaintRouting{
		OriginOfficeCode:      "40005CC",
		DestinationOfficeCode: "40006CC",
		CopyOfficeCodes:       "40007CC, 40008CC",
		AllTargetOfficeCodes:  "40006CC,40008CC,40005",
	}
	for _, officeCode := range []string{"40005CC", "40006CC", "40007CC", "40008CC", "40005"} {
		if !routing.InvolvesOffice(officeCode) {
			t.Errorf("InvolvesOffice(%q) = false, want true", officeCode)
		}
	}
}

func TestComplaintRoutingUsesExactOfficeTokens(t *testing.T) {
	routing := ComplaintRouting{
		CopyOfficeCodes:      "140005CC, 40005CC2",
		AllTargetOfficeCodes: "940005CC",
	}
	if routing.InvolvesOffice("40005CC") {
		t.Fatal("office code matched only a substring of another office code")
	}
}

func TestComplaintRoutingRejectsEmptyOfficeCode(t *testing.T) {
	routing := ComplaintRouting{OriginOfficeCode: "", AllTargetOfficeCodes: "40005CC"}
	if routing.InvolvesOffice("") || routing.InvolvesOffice("   ") {
		t.Fatal("empty office code must never match routing data")
	}
}
