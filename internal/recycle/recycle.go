package recycle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// Drive is one volume's contribution to the recycle bin.
type Drive struct {
	Drive     string  `json:"drive"`
	TotalMB   float64 `json:"total_mb"`
	ItemCount int64   `json:"item_count"`
}

// RecycleInfo describes the current contents of the recycle bin.
type RecycleInfo struct {
	TotalMB   float64 `json:"total_mb"`
	ItemCount int64   `json:"item_count"`
	PerDrive  []Drive `json:"per_drive"`
}

// EmptyResult is the outcome of a recycle bin empty.
type EmptyResult struct {
	DryRun bool        `json:"dry_run"`
	Ran    bool        `json:"ran"`
	Before RecycleInfo `json:"before"`
	After  RecycleInfo `json:"after,omitempty"`
	Output string      `json:"output,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// ThumbnailInfo describes the Explorer thumbnail/icon cache DB files.
type ThumbnailInfo struct {
	Path      string   `json:"path"`
	TotalMB   float64  `json:"total_mb"`
	FileCount int64    `json:"file_count"`
	Files     []string `json:"files"`
}

// ThumbnailClearResult is the outcome of clearing thumbnail caches.
type ThumbnailClearResult struct {
	DryRun    bool          `json:"dry_run"`
	Before    ThumbnailInfo `json:"before"`
	RemovedMB float64       `json:"removed_mb"`
	Locked    []string      `json:"locked,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// TempPath is one temp folder and its size.
type TempPath struct {
	Path      string  `json:"path"`
	Exists    bool    `json:"exists"`
	SizeMB    float64 `json:"size_mb"`
	FileCount int64   `json:"file_count"`
}

// TempInfo aggregates the system temp folders.
type TempInfo struct {
	TotalMB   float64    `json:"total_mb"`
	FileCount int64      `json:"file_count"`
	Paths     []TempPath `json:"paths"`
}

// CleanTempResult is the outcome of a temp folder sweep.
type CleanTempResult struct {
	DryRun    bool     `json:"dry_run"`
	Before    TempInfo `json:"before"`
	After     TempInfo `json:"after,omitempty"`
	RemovedMB float64  `json:"removed_mb"`
	Failed    []string `json:"failed,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// sizeOf walks a path and returns its on-disk size in MB and file count.
func sizeOf(path string) (float64, int64) {
	var total int64
	var count int64
	filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() {
			total += fi.Size()
			count++
		}
		return nil
	})
	return float64(total) / (1024 * 1024), count
}

func driveRoots() []string {
	var roots []string
	driveBits, err := windows.GetLogicalDrives()
	if err != nil {
		return roots
	}
	for i := 0; i < 26; i++ {
		if driveBits&(1<<uint(i)) != 0 {
			roots = append(roots, string(rune('A'+i))+`:\`)
		}
	}
	return roots
}

// Info sizes every drive's recycle bin ($Recycle.Bin\<SID>).
func Info() RecycleInfo {
	var info RecycleInfo
	for _, root := range driveRoots() {
		bin := filepath.Join(root, `$Recycle.Bin`)
		if fi, err := os.Stat(bin); err != nil || !fi.IsDir() {
			continue
		}
		mb, count := sizeOf(bin)
		if count == 0 {
			continue
		}
		info.PerDrive = append(info.PerDrive, Drive{Drive: root[:2], TotalMB: mb, ItemCount: count})
		info.TotalMB += mb
		info.ItemCount += count
	}
	sort.Slice(info.PerDrive, func(i, j int) bool { return info.PerDrive[i].TotalMB > info.PerDrive[j].TotalMB })
	return info
}

// Empty clears the recycle bin on every drive. Dry run only reports.
func Empty(ctx context.Context, dryRun bool, timeoutSec int) EmptyResult {
	before := Info()
	res := EmptyResult{DryRun: dryRun, Before: before}
	if dryRun {
		return res
	}
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	ctxT, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctxT, "powershell.exe", "-NoProfile", "-NonInteractive",
		"-Command", "Clear-RecycleBin -Force -ErrorAction SilentlyContinue")
	out, err := cmd.CombinedOutput()
	res.Ran = true
	res.Output = strings.TrimSpace(string(out))
	if ctxT.Err() == context.DeadlineExceeded {
		res.Error = "timed out"
	} else if err != nil {
		res.Error = err.Error()
	} else {
		res.After = Info()
	}
	return res
}

func explorerCacheDir() string {
	loc := os.Getenv("LOCALAPPDATA")
	if loc == "" {
		return ""
	}
	p := filepath.Join(loc, "Microsoft", "Windows", "Explorer")
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		return ""
	}
	return p
}

func thumbCacheFiles(dir string) (files []string, sizes map[string]int64, total int64) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.db"))
	if err != nil {
		return files, sizes, total
	}
	sizes = make(map[string]int64)
	for _, m := range matches {
		base := strings.ToLower(filepath.Base(m))
		if !strings.HasPrefix(base, "thumbcache_") && !strings.HasPrefix(base, "iconcache_") {
			continue
		}
		if fi, err := os.Stat(m); err == nil {
			sizes[m] = fi.Size()
			files = append(files, m)
			total += fi.Size()
		}
	}
	sort.Strings(files)
	return files, sizes, total
}

// Thumbnails reports the Explorer thumbnail and icon cache size.
func Thumbnails() ThumbnailInfo {
	dir := explorerCacheDir()
	if dir == "" {
		return ThumbnailInfo{}
	}
	files, _, total := thumbCacheFiles(dir)
	return ThumbnailInfo{
		Path:      dir,
		TotalMB:   float64(total) / (1024 * 1024),
		FileCount: int64(len(files)),
		Files:     files,
	}
}

// ClearThumbnails deletes the Explorer thumbnail/icon cache DB files. Files
// that Explorer has locked are reported, not fatal; they rebuild on demand.
func ClearThumbnails(dryRun bool) ThumbnailClearResult {
	before := Thumbnails()
	res := ThumbnailClearResult{DryRun: dryRun, Before: before}
	if dryRun {
		return res
	}
	files, sizes, _ := thumbCacheFiles(before.Path)
	var locked []string
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			locked = append(locked, f)
			continue
		}
		res.RemovedMB += float64(sizes[f]) / (1024 * 1024)
	}
	if len(locked) > 0 {
		res.Locked = locked
		res.Error = "some thumbnail cache files were locked by Explorer and remain"
	}
	return res
}

func tempPaths() []TempPath {
	seen := map[string]bool{}
	var paths []TempPath
	add := func(p string) {
		p = strings.TrimSpace(os.ExpandEnv(p))
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		tp := TempPath{Path: p}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			tp.Exists = true
			tp.SizeMB, tp.FileCount = sizeOf(p)
		}
		paths = append(paths, tp)
	}
	for _, env := range []string{"TEMP", "TMP"} {
		add(os.Getenv(env))
	}
	add(filepath.Join(os.Getenv("SystemRoot"), "Temp"))
	return paths
}

// Temp reports the size of the user and system temp folders.
func Temp() TempInfo {
	var info TempInfo
	info.Paths = tempPaths()
	for _, p := range info.Paths {
		if p.Exists {
			info.TotalMB += p.SizeMB
			info.FileCount += p.FileCount
		}
	}
	return info
}

func remove(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	time.Sleep(300 * time.Millisecond)
	return os.RemoveAll(path)
}

// CleanTemp removes the contents (not the folders themselves) of the user and
// system temp locations. Locked files are skipped and reported.
func CleanTemp(dryRun bool) CleanTempResult {
	before := Temp()
	res := CleanTempResult{DryRun: dryRun, Before: before}
	if dryRun {
		return res
	}
	var failed []string
	for _, p := range before.Paths {
		if !p.Exists {
			continue
		}
		entries, err := os.ReadDir(p.Path)
		if err != nil {
			failed = append(failed, p.Path)
			continue
		}
		for _, e := range entries {
			child := filepath.Join(p.Path, e.Name())
			if err := remove(child); err != nil {
				failed = append(failed, child)
			}
		}
	}
	res.After = Temp()
	res.RemovedMB = before.TotalMB - res.After.TotalMB
	if res.RemovedMB < 0 {
		res.RemovedMB = 0
	}
	if len(failed) > 0 {
		res.Failed = failed
		res.Error = "some temp files were locked and remain"
	}
	return res
}
