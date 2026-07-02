//go:build windows

package winsandbox

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// lowLabelSDDL is a security descriptor carrying a single mandatory-label ACE:
//
//	ML  = SYSTEM_MANDATORY_LABEL_ACE
//	OICI = object-inherit + container-inherit (new files/subdirs inherit it)
//	NW  = SYSTEM_MANDATORY_LABEL_NO_WRITE_UP (a Low process may write here, but
//	      cannot write "up" into higher-integrity objects)
//	LW  = Low mandatory level SID
//
// Applying it to the app-data directory is what lets the Low-integrity worker
// write there while the OS blocks writes anywhere else.
const lowLabelSDDL = "S:(ML;OICI;NW;;;LW)"

// SetLowLabel sets the Low mandatory-integrity label on the directory at path so
// a Low-integrity process can write into it. It must be called by a
// Medium-integrity process (the launcher): a Low process cannot raise or set
// integrity labels. It is idempotent — safe to re-run on every launch.
func SetLowLabel(path string) error {
	sd, err := windows.SecurityDescriptorFromString(lowLabelSDDL)
	if err != nil {
		return fmt.Errorf("parse low-label SDDL: %w", err)
	}
	sacl, _, err := sd.SACL()
	if err != nil {
		return fmt.Errorf("extract SACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.LABEL_SECURITY_INFORMATION,
		nil, nil, nil,
		sacl,
	); err != nil {
		return fmt.Errorf("set low integrity label on %s: %w", path, err)
	}
	return nil
}
