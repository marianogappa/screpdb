//go:build windows

package selfupdate

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SelfInstall moves a browser-downloaded binary into
// %LOCALAPPDATA%\Programs\screpdb, adds Start Menu and Desktop shortcuts, and
// launches the installed copy. It reports whether that copy was launched, in
// which case the caller must exit. Any failure leaves the app running from
// where it is.
func SelfInstall() bool {
	self, err := executablePath()
	if err != nil {
		return false
	}
	downloads, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
	if err != nil {
		return false
	}
	local, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return false
	}
	installDir := filepath.Join(local, "Programs", "screpdb")
	if !shouldSelfInstall(self, downloads, installDir) {
		return false
	}

	target := filepath.Join(installDir, installedExeName)
	if err := copyBinary(self, target); err != nil {
		log.Printf("self-install: copy to %s failed: %v", target, err)
		return false
	}
	for _, folder := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Programs, windows.FOLDERID_Desktop} {
		dir, err := windows.KnownFolderPath(folder, 0)
		if err != nil {
			continue
		}
		if err := createShortcut(filepath.Join(dir, "screpdb.lnk"), target, installDir); err != nil {
			log.Printf("self-install: shortcut in %s failed: %v", dir, err)
		}
	}

	cmd := exec.Command(target, os.Args[1:]...)
	cmd.Dir = installDir
	cmd.Env = append(os.Environ(), movedFromEnv+"="+self)
	if err := cmd.Start(); err != nil {
		log.Printf("self-install: launching %s failed: %v", target, err)
		return false
	}
	log.Printf("self-install: moved %s to %s", self, target)
	return true
}

// copyBinary writes a fresh copy of src at dst. A fresh write carries no
// Mark-of-the-Web, so SmartScreen does not prompt again for the installed copy.
// An existing dst may be running, so it is renamed aside (Windows allows that)
// and CleanupOldBinary removes it on a later launch.
func copyBinary(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(dir, ".screpdb-install-*")
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(tmp, in)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write copy: %v %v", copyErr, closeErr)
	}
	if _, err := os.Stat(dst); err == nil {
		old := filepath.Join(dir, "."+filepath.Base(dst)+".old")
		_ = os.Remove(old)
		if err := os.Rename(dst, old); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")

	clsidShellLink  = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIShellLinkW  = windows.GUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIPersistFile = windows.GUID{Data1: 0x0000010B, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// Vtable slots of IShellLinkW and IPersistFile, counted from IUnknown's three.
const (
	slotQueryInterface      = 0
	slotRelease             = 2
	slotSetDescription      = 7
	slotSetWorkingDirectory = 9
	slotSetPath             = 20
	slotPersistSave         = 6
)

// createShortcut writes a .lnk at lnkPath pointing at target, through the
// Shell's own IShellLink so the file is exactly what Explorer would write.
func createShortcut(lnkPath, target, workDir string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && err != syscall.Errno(windows.S_FALSE) {
		return err
	}
	defer windows.CoUninitialize()

	var link *comObject
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)), 0, windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if hr != 0 {
		return fmt.Errorf("CoCreateInstance: 0x%x", hr)
	}
	defer comCall(link, slotRelease)

	for _, step := range []struct {
		slot  int
		value string
	}{{slotSetPath, target}, {slotSetWorkingDirectory, workDir}, {slotSetDescription, "screpdb dashboard"}} {
		p, err := windows.UTF16PtrFromString(step.value)
		if err != nil {
			return err
		}
		if hr := comCall(link, step.slot, uintptr(unsafe.Pointer(p))); hr != 0 {
			return fmt.Errorf("IShellLink slot %d: 0x%x", step.slot, hr)
		}
	}

	var persist *comObject
	if hr := comCall(link, slotQueryInterface, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&persist))); hr != 0 {
		return fmt.Errorf("QueryInterface IPersistFile: 0x%x", hr)
	}
	defer comCall(persist, slotRelease)
	p, err := windows.UTF16PtrFromString(lnkPath)
	if err != nil {
		return err
	}
	if hr := comCall(persist, slotPersistSave, uintptr(unsafe.Pointer(p)), 1); hr != 0 {
		return fmt.Errorf("IPersistFile.Save: 0x%x", hr)
	}
	return nil
}

type comObject struct {
	vtbl *[32]uintptr
}

func comCall(obj *comObject, slot int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(obj.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)...)
	return hr
}
