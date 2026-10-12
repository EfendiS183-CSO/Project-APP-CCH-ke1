package complaints

import "strings"

type ComplaintRouting struct {
	OriginOfficeCode      string
	DestinationOfficeCode string
	CopyOfficeCodes       string
	AllTargetOfficeCodes  string
}

func (routing ComplaintRouting) InvolvesOffice(officeCode string) bool {
	officeCode = strings.TrimSpace(officeCode)
	if officeCode == "" {
		return false
	}
	if officeCode == strings.TrimSpace(routing.OriginOfficeCode) || officeCode == strings.TrimSpace(routing.DestinationOfficeCode) {
		return true
	}
	return containsOfficeToken(routing.CopyOfficeCodes, officeCode) || containsOfficeToken(routing.AllTargetOfficeCodes, officeCode)
}

func containsOfficeToken(officeCodes string, officeCode string) bool {
	for _, candidate := range strings.Split(officeCodes, ",") {
		if strings.TrimSpace(candidate) == officeCode {
			return true
		}
	}
	return false
}
