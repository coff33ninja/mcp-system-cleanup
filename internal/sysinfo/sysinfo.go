package sysinfo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Volume describes one drive's capacity and usage.
type Volume struct {
	ID         string  `json:"id"`
	MountPoint string  `json:"mount_point"`
	TotalBytes uint64  `json:"total_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	UsedPct    float64 `json:"used_percent"`
}

// Volumes lists all drives with capacity info using the Windows API.
func Volumes() []Volume {
	var out []Volume
	driveBits, err := windows.GetLogicalDrives()
	if err != nil {
		return out
	}
	for i := 0; i < 26; i++ {
		if driveBits&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + ":\\"
		var total, free uint64
		_ = windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(root), &free, &total, nil)
		if total == 0 {
			continue
		}
		used := total - free
		v := Volume{
			ID:         root[:2],
			MountPoint: root,
			TotalBytes: total,
			FreeBytes:  free,
			UsedBytes:  used,
		}
		if total > 0 {
			v.UsedPct = float64(used) / float64(total) * 100
		}
		out = append(out, v)
	}
	return out
}

// DirInfo describes one path's size on disk.
type DirInfo struct {
	Path      string  `json:"path"`
	Exists    bool    `json:"exists"`
	SizeMB    float64 `json:"size_mb"`
	FileCount int64   `json:"file_count"`
}

// DirSizes computes sizes for a list of paths. Each path may be a file or dir.
func DirSizes(paths []string) []DirInfo {
	var out []DirInfo
	for _, p := range paths {
		info := DirInfo{Path: p}
		fi, err := os.Stat(p)
		if err != nil || fi == nil {
			out = append(out, info)
			continue
		}
		info.Exists = true
		if !fi.IsDir() {
			info.SizeMB = float64(fi.Size()) / (1024 * 1024)
			out = append(out, info)
			continue
		}
		var total int64
		var count int64
		filepath.Walk(p, func(_ string, f os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !f.IsDir() {
				total += f.Size()
				count++
			}
			return nil
		})
		info.SizeMB = float64(total) / (1024 * 1024)
		info.FileCount = count
		out = append(out, info)
	}
	return out
}

// HibernateInfo reports whether hibernation is available and the size of the
// hibernation file (hiberfil.sys).
type HibernateInfo struct {
	HiberfilPath string  `json:"hiberfil_path"`
	HiberfilMB   float64 `json:"hiberfil_mb"`
	Available    bool    `json:"available"`
}

// Hibernate reports hibernate state.
func Hibernate() HibernateInfo {
	info := HibernateInfo{HiberfilPath: `C:\hiberfil.sys`}
	if fi, err := os.Stat(info.HiberfilPath); err == nil {
		info.Available = true
		info.HiberfilMB = float64(fi.Size()) / (1024 * 1024)
	}
	return info
}

// SetHibernate toggles hibernation on/off via powercfg. Requires admin.
func SetHibernate(on bool) (string, error) {
	arg := "/hibernate off"
	if on {
		arg = "/hibernate on"
	}
	cmd := exec.Command("powercfg", strings.Fields(arg)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("powercfg %s: %w", arg, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DISMResult is the outcome of a component-store cleanup.
type DISMResult struct {
	Ran       bool   `json:"ran"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

// DISMCleanup runs `DISM /Online /Cleanup-Image /StartComponentCleanup`.
// Requires admin. The timeout is generous (component cleanup can take minutes).
func DISMCleanup(ctx context.Context, timeoutSec int) DISMResult {
	if timeoutSec <= 0 {
		timeoutSec = 900
	}
	start := time.Now()
	ctxT, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxT, "Dism.exe",
		"/Online", "/Cleanup-Image", "/StartComponentCleanup")
	out, err := cmd.CombinedOutput()

	res := DISMResult{Ran: true, ElapsedMs: time.Since(start).Milliseconds(), Output: strings.TrimSpace(string(out))}
	if ctxT.Err() == context.DeadlineExceeded {
		res.Error = "timed out"
	} else if err != nil {
		res.Error = err.Error()
	}
	return res
}

// DISMResetBase runs `DISM /Online /Cleanup-Image /StartComponentCleanup
// /ResetBase`, which permanently removes superseded components from WinSxS.
// More aggressive than DISMCleanup. Requires admin.
func DISMResetBase(ctx context.Context, timeoutSec int) DISMResult {
	if timeoutSec <= 0 {
		timeoutSec = 900
	}
	start := time.Now()
	ctxT, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxT, "Dism.exe",
		"/Online", "/Cleanup-Image", "/StartComponentCleanup", "/ResetBase")
	out, err := cmd.CombinedOutput()

	res := DISMResult{Ran: true, ElapsedMs: time.Since(start).Milliseconds(), Output: strings.TrimSpace(string(out))}
	if ctxT.Err() == context.DeadlineExceeded {
		res.Error = "timed out"
	} else if err != nil {
		res.Error = err.Error()
	}
	return res
}

// OSInfo describes the installed Windows version.
type OSInfo struct {
	ProductName    string `json:"product_name"`
	DisplayVersion string `json:"display_version"`
	CurrentBuild   string `json:"current_build"`
	Arch           string `json:"arch"`
}

// OSVersion reads the installed Windows version from the registry.
func OSVersion() OSInfo {
	var o OSInfo
	o.Arch = runtime.GOARCH
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return o
	}
	defer k.Close()
	o.ProductName, _, _ = k.GetStringValue("ProductName")
	o.DisplayVersion, _, _ = k.GetStringValue("DisplayVersion")
	o.CurrentBuild, _, _ = k.GetStringValue("CurrentBuildNumber")
	if o.CurrentBuild == "" {
		o.CurrentBuild, _, _ = k.GetStringValue("CurrentBuild")
	}
	return o
}

// MemoryInfo describes physical RAM usage.
type MemoryInfo struct {
	TotalBytes uint64  `json:"total_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	UsedPct    float64 `json:"used_percent"`
}

// memoryStatusEx mirrors the Win32 MEMORYSTATUSEX structure.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// Memory reports physical RAM usage via GlobalMemoryStatusEx.
func Memory() MemoryInfo {
	var ms memoryStatusEx
	ms.length = uint32(unsafe.Sizeof(ms))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		return MemoryInfo{}
	}
	used := ms.totalPhys - ms.availPhys
	m := MemoryInfo{TotalBytes: ms.totalPhys, FreeBytes: ms.availPhys, UsedBytes: used}
	if ms.totalPhys > 0 {
		m.UsedPct = float64(used) / float64(ms.totalPhys) * 100
	}
	return m
}
