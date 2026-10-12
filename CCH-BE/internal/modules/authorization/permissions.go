package authorization

type Action int64

const (
	ActionAdd      Action = 1
	ActionDelete   Action = 2
	ActionEdit     Action = 4
	ActionList     Action = 8
	ActionAdmin    Action = 16
	ActionView     Action = 32
	ActionSearch   Action = 64
	ActionImport   Action = 128
	ActionLookup   Action = 256
	ActionPush     Action = 512
	ActionExport   Action = 1024
	ActionPrint    Action = 2048
	ActionExcel    Action = 4096
	ActionWord     Action = 8192
	ActionHTML     Action = 16384
	ActionXML      Action = 32768
	ActionCSV      Action = 65536
	ActionPDF      Action = 131072
	ActionEmail    Action = 262144
	AllPermissions int64  = 524287
)

func EffectiveMask(roleMasks []int64, systemAdministrator bool, tokenMask int64) int64 {
	var effectiveMask int64
	if systemAdministrator {
		effectiveMask = AllPermissions
	} else {
		for _, roleMask := range roleMasks {
			effectiveMask |= roleMask
		}
	}

	if tokenMask > 0 {
		effectiveMask &= tokenMask
	}
	return effectiveMask
}

func Allows(mask int64, action Action) bool {
	return mask&int64(action) == int64(action)
}
