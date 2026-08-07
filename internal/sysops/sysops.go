package sysops

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// StartupItem is one entry that runs at startup.
type StartupItem struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Location string `json:"location"`
}

// StartupInventory enumerates Run/RunOnce registry keys (HKLM + HKCU) and the
// user/all-users Startup folders.
func StartupInventory() []StartupItem {
	var items []StartupItem
	runKeys := []struct {
		key  registry.Key
		path string
		loc  string
	}{
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, "HKLM\\...\\Run"},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, "HKLM\\...\\RunOnce"},
		{registry.LOCAL_MACHINE, `Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Run`, "HKLM\\Wow6432Node\\...\\Run"},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, "HKCU\\...\\Run"},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, "HKCU\\...\\RunOnce"},
	}
	for _, rk := range runKeys {
		k, err := registry.OpenKey(rk.key, rk.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if names, err := k.ReadValueNames(0); err == nil {
			for _, n := range names {
				if v, _, _ := k.GetStringValue(n); v != "" {
					items = append(items, StartupItem{Name: n, Command: v, Location: rk.loc})
				}
			}
		}
		k.Close()
	}

	startupDirs := []struct {
		env string
		loc string
	}{
		{"APPDATA", "User startup folder"},
		{"ProgramData", "All-users startup folder"},
	}
	for _, sd := range startupDirs {
		base := os.Getenv(sd.env)
		if base == "" {
			continue
		}
		dir := filepath.Join(base, `Microsoft\Windows\Start Menu\Programs\Startup`)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			items = append(items, StartupItem{Name: e.Name(), Command: filepath.Join(dir, e.Name()), Location: sd.loc})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Location == items[j].Location {
			return items[i].Name < items[j].Name
		}
		return items[i].Location < items[j].Location
	})
	return items
}

// Pagefile is one configured page file.
type Pagefile struct {
	Path   string  `json:"path"`
	Exists bool    `json:"exists"`
	SizeMB float64 `json:"size_mb"`
}

// Pagefiles reads the configured paging files and their current sizes.
func Pagefiles() []Pagefile {
	var out []Pagefile
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management`, registry.QUERY_VALUE)
	if err != nil {
		return out
	}
	defer k.Close()
	vals, _, err := k.GetStringsValue("PagingFiles")
	if err != nil {
		return out
	}
	seen := map[string]bool{}
	for _, v := range vals {
		v = strings.TrimSpace(os.ExpandEnv(v))
		// System-managed pagefiles are recorded as "?:\pagefile.sys".
		if strings.HasPrefix(v, "?:\\") {
			sd := os.Getenv("SystemDrive")
			if sd == "" {
				sd = "C:"
			}
			v = sd + "\\" + strings.TrimPrefix(v, "?:\\")
		}
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		pf := Pagefile{Path: filepath.Clean(v)}
		if fi, err := os.Stat(pf.Path); err == nil {
			pf.Exists = true
			pf.SizeMB = float64(fi.Size()) / (1024 * 1024)
		}
		out = append(out, pf)
	}
	return out
}

// WUCacheInfo describes the Windows Update download cache.
type WUCacheInfo struct {
	Path      string  `json:"path"`
	Exists    bool    `json:"exists"`
	TotalMB   float64 `json:"total_mb"`
	FileCount int64   `json:"file_count"`
}

func wuCacheDir() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, `SoftwareDistribution\Download`)
}

// WUCache reports the size of the Windows Update download cache.
func WUCache() WUCacheInfo {
	dir := wuCacheDir()
	info := WUCacheInfo{Path: dir}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return info
	}
	info.Exists = true
	var total int64
	var count int64
	filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() {
			total += fi.Size()
			count++
		}
		return nil
	})
	info.TotalMB = float64(total) / (1024 * 1024)
	info.FileCount = count
	return info
}

// WUCacheClearResult is the outcome of clearing the WU download cache.
type WUCacheClearResult struct {
	DryRun    bool        `json:"dry_run"`
	Before    WUCacheInfo `json:"before"`
	After     WUCacheInfo `json:"after,omitempty"`
	RemovedMB float64     `json:"removed_mb"`
	Failed    []string    `json:"failed,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// ClearWUCache removes the contents of the Windows Update download cache.
// Requires admin. Files locked by the wuauserv service are reported.
func ClearWUCache(ctx context.Context, dryRun bool, timeoutSec int) WUCacheClearResult {
	before := WUCache()
	res := WUCacheClearResult{DryRun: dryRun, Before: before}
	if dryRun || !before.Exists {
		return res
	}
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	ctxT, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	entries, err := os.ReadDir(before.Path)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	var failed []string
	for _, e := range entries {
		if ctxT.Err() != nil {
			res.Error = "timed out or canceled"
			break
		}
		child := filepath.Join(before.Path, e.Name())
		if err := remove(child); err != nil {
			failed = append(failed, child)
		}
	}
	res.After = WUCache()
	res.RemovedMB = before.TotalMB - res.After.TotalMB
	if res.RemovedMB < 0 {
		res.RemovedMB = 0
	}
	if len(failed) > 0 {
		res.Failed = failed
		res.Error = "some Windows Update files were locked by the Update service"
	}
	return res
}

func remove(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	time.Sleep(300 * time.Millisecond)
	return os.RemoveAll(path)
}
