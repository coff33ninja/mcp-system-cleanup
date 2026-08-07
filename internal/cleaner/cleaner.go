package cleaner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Category is a top-level group of cache locations, inspired by the
// local cleanup tools used to keep this system tidy.
type Category struct {
	Num  int    `json:"num"`
	Name string `json:"name"`
}

// Target is a filesystem location that can be removed (a folder or file).
type Target struct {
	Path     string  `json:"path"`
	Label    string  `json:"label"`
	Category int     `json:"category"`
	Exists   bool    `json:"exists"`
	SizeMB   float64 `json:"size_mb"`
}

// PurgeCmd is a tool-managed cache purge (pip cache purge, npm cache clean, ...).
type PurgeCmd struct {
	Label    string `json:"label"`
	Command  string `json:"command"`
	Category int    `json:"category"`
}

// Options control a cleanup run.
type Options struct {
	DryRun          bool  `json:"dry_run"`
	MinSizeMB       int   `json:"min_size_mb"`
	OlderThanDays   int   `json:"older_than_days"`
	Categories      []int `json:"categories"`
	PurgeTimeoutSec int   `json:"purge_timeout_sec"` // 0 => 60
}

// Result describes the outcome of one removal attempt.
type Result struct {
	Target    Target  `json:"target"`
	RemovedMB float64 `json:"removed_mb"`
	Status    string  `json:"status"` // removed | skipped | locked | missing | dry-run
	ElapsedMs int64   `json:"elapsed_ms"`
	Error     string  `json:"error,omitempty"`
}

// RunReport is the aggregate of a cleanup run.
type RunReport struct {
	FreedMB   float64  `json:"freed_mb"`
	SkippedMB float64  `json:"skipped_mb"`
	TotalMB   float64  `json:"total_mb"`
	Results   []Result `json:"results"`
}

// Categories returns the category list.
func Categories() []Category {
	return []Category{
		{1, "Tool-managed cache purges"},
		{2, "Language version managers & SDK toolchains"},
		{3, "ML / AI model and dataset caches"},
		{4, "IDE and build-tool caches"},
		{5, "Language runtime caches and leftovers"},
		{6, "Container, virtualization & cloud tools"},
		{7, "Browser development caches"},
		{8, "Database tools"},
		{9, "Game development engines"},
		{10, "AppData extras (Roaming and Local)"},
	}
}

func catName(n int) string {
	for _, c := range Categories() {
		if c.Num == n {
			return c.Name
		}
	}
	return "Unknown"
}

func inCats(opts Options, n int) bool {
	if len(opts.Categories) == 0 {
		return true
	}
	for _, c := range opts.Categories {
		if c == n {
			return true
		}
	}
	return false
}

// dirSizeMB computes the total size of a path in MB.
func dirSizeMB(path string) float64 {
	var total int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return float64(total) / (1024 * 1024)
}

func ageDays(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 999999
	}
	return int(time.Since(info.ModTime()).Hours() / 24)
}

// Targets returns the full set of filesystem cache locations across all
// categories, resolved against the current user's environment.
func Targets() []Target {
	u := os.Getenv("USERPROFILE")
	app := os.Getenv("APPDATA")
	loc := os.Getenv("LOCALAPPDATA")
	if u == "" {
		if runtime.GOOS == "windows" {
			u = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		} else {
			u, _ = os.UserHomeDir()
		}
	}

	// helper: a target that is skipped when the parent env var is empty
	t := func(path, label string, cat int) Target {
		if path == "" {
			return Target{Path: "", Label: label, Category: cat, Exists: false}
		}
		p := filepath.Clean(os.ExpandEnv(path))
		_, err := os.Stat(p)
		return Target{Path: p, Label: label, Category: cat, Exists: err == nil}
	}

	join := func(parts ...string) string {
		if len(parts) == 0 || parts[0] == "" {
			return ""
		}
		return filepath.Join(parts...)
	}

	var list []Target

	// ============ Category 2: version managers & SDK toolchains ============
	add := func(path, label string, cat int) {
		list = append(list, t(path, label, cat))
	}
	add(join(u, ".rustup"), "Rust -- rustup toolchain manager", 2)
	add(join(u, ".cargo"), "Rust -- Cargo registry & tools", 2)
	add(join(u, ".pyenv"), "Python -- pyenv version manager", 2)
	add(join(u, ".conda"), "Python -- Conda state", 2)
	add(join(u, ".anaconda"), "Python -- Anaconda leftover", 2)
	add(join(u, ".micromamba"), "Python -- Micromamba pkgs", 2)
	add(join(u, ".local", "pipx"), "Python -- pipx app installs", 2)
	add(join(u, ".rye"), "Python -- Rye package manager", 2)
	add(join(u, ".pdm"), "Python -- PDM package manager", 2)
	add(join(loc, "pdm", "cache"), "Python -- PDM cache", 2)
	add(join(u, ".nvm"), "Node -- nvm version manager", 2)
	add(join(u, ".fnm"), "Node -- fnm version manager", 2)
	add(join(loc, "fnm_multishells"), "Node -- fnm shell spawns", 2)
	add(join(u, ".volta"), "Node -- Volta toolchain", 2)
	add(join(loc, "Volta"), "Node -- Volta local cache", 2)
	add(join(u, ".sdkman"), "Java -- SDKMAN candidates", 2)
	add(join(u, ".jabba"), "Java -- Jabba JDK manager", 2)
	add(join(u, ".jenv"), "Java -- jenv shims", 2)
	add(join(u, ".m2", "repository"), "Java -- Maven local repository", 2)
	add(join(u, ".ivy2", "cache"), "Java/Scala -- Ivy2 artifact cache", 2)
	add(join(u, ".sbt"), "Scala -- SBT launcher cache", 2)
	add(join(u, ".gradle"), "JVM -- Gradle caches & wrapper dists", 2)
	add(join(u, ".gradle-enterprise"), "Gradle -- Enterprise plugin cache", 2)
	add(join(u, ".dotnet"), ".NET -- SDK leftover state", 2)
	add(join(loc, "NuGet", "v3-cache"), ".NET -- NuGet v3 HTTP cache", 2)
	add(join(u, ".nuget", "packages"), ".NET -- NuGet global packages", 2)
	add(join(u, ".rbenv"), "Ruby -- rbenv version manager", 2)
	add(join(u, ".rvm"), "Ruby -- RVM", 2)
	add(join(u, ".bundle", "cache"), "Ruby -- Bundler gem cache", 2)
	add(join(u, ".gem"), "Ruby -- gem home", 2)
	add(join(u, "go", "pkg", "mod", "cache"), "Go -- module download cache", 2)
	add(join(loc, "go-build"), "Go -- build cache", 2)
	add(join(app, "Composer", "cache"), "PHP -- Composer cache", 2)
	add(join(u, ".pub-cache"), "Dart/Flutter -- pub package cache", 2)
	add(join(loc, ".dartServer"), "Dart -- language server cache", 2)
	add(join(app, ".dart"), "Dart -- user config remnants", 2)
	add(join(app, ".dart-tool"), "Dart -- tool remnants", 2)
	add(join(loc, "Pub", "Cache"), "Dart -- Pub cache (Windows)", 2)
	add(join(u, ".stack"), "Haskell -- Stack GHC snapshots", 2)
	add(join(u, ".ghcup"), "Haskell -- GHCup toolchain", 2)
	add(join(u, ".cabal"), "Haskell -- Cabal packages", 2)
	add(join(u, ".espressif"), "ESP32 -- ESP-IDF SDK", 2)
	add(join(u, ".platformio"), "Embedded -- PlatformIO SDK", 2)
	add(join(u, ".arduinoIDE"), "Arduino -- IDE 2.x data", 2)
	add(join(u, ".arduino-create"), "Arduino -- Create agent", 2)
	add(join(u, ".windows-build-tools"), "Node -- Windows build tools", 2)

	// ============ Category 3: ML / AI model and dataset caches ============
	add(join(u, ".cache", "huggingface"), "HuggingFace -- model & dataset cache", 3)
	add(join(u, ".cache", "torch"), "PyTorch -- extension cache", 3)
	add(join(loc, "torch"), "PyTorch -- extension cache (Local)", 3)
	add(join(u, ".EasyOCR"), "EasyOCR -- model cache", 3)
	add(join(u, ".keras"), "Keras -- dataset cache", 3)
	add(join(u, ".triton"), "Triton -- JIT kernel cache", 3)
	add(join(u, ".unsloth"), "Unsloth -- finetuning cache", 3)
	add(join(u, ".olive-cache"), "ONNX Olive -- optimisation cache", 3)
	add(join(u, ".lmstudio"), "LM Studio -- model downloads", 3)
	add(join(u, ".ollama"), "Ollama -- model blobs", 3)
	add(join(u, ".webui"), "WebUI -- cache", 3)
	add(join(u, ".nemo"), "NVIDIA NeMo -- model cache", 3)
	add(join(u, ".cache", "wandb"), "Weights & Biases -- experiment cache", 3)
	add(join(u, ".cache", "tensorboard"), "TensorBoard -- log cache", 3)
	add(join(u, ".comet"), "Comet ML -- experiment cache", 3)

	// ============ Category 4: IDE and build-tool caches ============
	add(join(u, ".eclipse"), "Eclipse -- workspace metadata", 4)
	add(join(u, ".cmake-js"), "cmake-js -- native build cache", 4)
	add(join(u, ".javacpp"), "JavaCPP -- extracted native libs", 4)
	add(join(u, ".buildcache"), "buildcache -- compiler cache", 4)
	add(join(u, ".redhat"), "Red Hat LS -- language server cache", 4)
	add(join(u, ".lemminx"), "LemMinX -- XML LS cache", 4)
	add(join(app, "Code", "CachedData"), "VS Code -- compiled extension cache", 4)
	add(join(app, "Code", "Cache"), "VS Code -- HTTP asset cache", 4)
	add(join(app, "Code", "GPUCache"), "VS Code -- GPU shader cache", 4)
	add(join(app, "Code", "logs"), "VS Code -- logs", 4)

	// ============ Category 5: language runtime caches ============
	add(join(u, ".ipython"), "Python -- IPython history & cache", 5)
	add(join(u, ".jupyter"), "Python -- Jupyter runtime cache", 5)
	add(join(u, ".ipynb_checkpoints"), "Python -- Jupyter checkpoint files", 5)
	add(join(u, ".matplotlib"), "Python -- Matplotlib font cache", 5)
	add(join(u, ".streamlit"), "Python -- Streamlit cache", 5)
	add(join(u, ".kivy"), "Python -- Kivy framework cache", 5)
	add(join(u, ".pydev_vscode"), "Python -- PyDev VS Code cache", 5)
	add(join(u, ".pydevd_vscode"), "Python -- PyDevd debugger cache", 5)
	add(join(u, ".idlerc"), "Python -- IDLE config & history", 5)
	add(join(u, ".qt_material"), "Python -- Qt Material theme cache", 5)
	add(join(u, ".mypy_cache"), "Python -- MyPy type checker cache", 5)
	add(join(u, ".pytest_cache"), "Python -- Pytest cache", 5)
	add(join(u, ".tox"), "Python -- Tox virtual environments", 5)
	add(join(u, ".nox"), "Python -- Nox session cache", 5)
	add(join(u, ".cache", "pip"), "Python -- pip cache (user)", 5)
	add(join(u, ".node-red"), "Node-RED -- userdir cache", 5)
	add(join(u, ".eslintcache"), "JavaScript -- ESLint cache", 5)
	add(join(u, ".prettiercache"), "JavaScript -- Prettier cache", 5)
	add(join(loc, "typescript"), "TypeScript -- language server cache", 5)
	add(join(u, ".nx"), "Nx -- monorepo cache", 5)
	add(join(u, ".rush"), "Rush -- monorepo cache", 5)
	add(join(loc, "turbo"), "Turborepo -- cache", 5)
	add(join(u, ".minikube"), "K8s -- Minikube VM cache", 5)
	add(join(u, ".kube", "cache"), "K8s -- kubectl HTTP cache", 5)
	add(join(u, ".kube", "cache", "discovery"), "K8s -- kubectl discovery cache", 5)
	add(join(u, ".goftp"), "GoFTP -- leftover cache", 5)
	add(join(u, ".swt"), "SWT -- native lib cache", 5)
	add(join(u, ".thumbnails"), "Thumbnails cache", 5)
	add(join(u, ".openshot_qt"), "OpenShot -- temp/cache", 5)

	// ============ Category 6: container, virtualization & cloud ============
	add(join(u, ".docker", "buildx"), "Docker -- Buildx cache", 6)
	add(join(u, ".podman"), "Podman -- cache", 6)
	add(join(loc, "Docker", "wsl", "data"), "Docker Desktop -- WSL data", 6)
	add(join(u, ".vagrant.d"), "Vagrant -- boxes", 6)
	add(join(u, ".aws"), "AWS CLI -- cache", 6)
	add(join(u, ".azure"), "Azure CLI -- cache", 6)
	add(join(u, ".config", "gcloud"), "Google Cloud SDK -- cache", 6)

	// ============ Category 7: browser development caches ============
	add(join(loc, "Microsoft", "Edge", "User Data", "Default", "Cache"), "Edge -- DevTools cache", 7)
	add(join(loc, "Google", "Chrome", "User Data", "Default", "Cache"), "Chrome -- DevTools cache", 7)
	add(join(loc, "BraveSoftware", "Brave-Browser", "User Data", "Default", "Cache"), "Brave -- DevTools cache", 7)

	// ============ Category 8: database tools ============
	add(join(u, ".mongodb"), "MongoDB -- Compass cache", 8)
	add(join(u, ".postgresql"), "PostgreSQL -- client cache", 8)
	add(join(loc, "DBeaverData", "workspace6", ".metadata"), "DBeaver -- workspace cache", 8)

	// ============ Category 9: game development engines ============
	add(join(loc, "UnrealEngine"), "Unreal Engine -- cache", 9)
	add(join(u, ".godot"), "Godot Engine -- cache", 9)
	add(join(app, "Unity"), "Unity -- cache", 9)

	// ============ Category 10: AppData extras ============
	add(join(loc, "pip", "Cache"), "pip -- LOCALAPPDATA cache", 10)
	add(join(app, "npm-cache"), "npm -- APPDATA cache", 10)
	add(join(loc, "Yarn", "Cache"), "Yarn -- LOCALAPPDATA cache", 10)
	add(join(loc, "pnpm-cache"), "pnpm -- LOCALAPPDATA cache", 10)
	add(join(loc, "uv", "cache"), "uv -- LOCALAPPDATA cache", 10)
	add(join(loc, "pypoetry", "Cache"), "Poetry -- LOCALAPPDATA cache", 10)
	add(join(app, "pypoetry", "Cache"), "Poetry -- APPDATA cache", 10)
	add(join(loc, "deno"), "Deno -- DENO_DIR (LOCALAPPDATA)", 10)
	add(join(loc, "bun"), "Bun -- LOCALAPPDATA cache", 10)
	add(join(loc, "zig"), "Zig -- LOCALAPPDATA cache", 10)
	add(join(loc, "Android", "Sdk", ".temp"), "Android SDK -- temp downloads", 10)
	add(join(loc, "Temp", "NuGetScratch"), "NuGet -- temp scratch dir", 10)

	return list
}

// PurgeCommands returns tool-managed purge commands (category 1), gated on
// whether the tool is present on PATH.
func PurgeCommands() []PurgeCmd {
	has := func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
	var cmds []PurgeCmd
	cmd := func(label, command string) {
		cmds = append(cmds, PurgeCmd{Label: label, Command: command, Category: 1})
	}
	if has("pip") {
		cmd("pip", "pip cache purge")
	}
	if has("pip3") {
		cmd("pip3", "pip3 cache purge")
	}
	if has("npm") {
		cmd("npm", "npm cache clean --force")
	}
	if has("yarn") {
		cmd("yarn", "yarn cache clean")
	}
	if has("pnpm") {
		cmd("pnpm", "pnpm store prune")
	}
	if has("bun") {
		cmd("bun", "bun pm cache rm")
	}
	if has("deno") {
		cmd("deno", "deno clean")
	}
	if has("cargo") {
		cmd("cargo", "cargo cache --autoclean")
	}
	if has("dotnet") {
		cmd("NuGet http-cache", "dotnet nuget locals http-cache --clear")
	}
	if has("dotnet") {
		cmd("NuGet temp", "dotnet nuget locals temp --clear")
	}
	if has("conda") {
		cmd("conda", "conda clean --all -y")
	}
	if has("uv") {
		cmd("uv", "uv cache clean")
	}
	if has("flutter") {
		cmd("Flutter pub", "flutter pub cache clean")
	}
	if has("dart") {
		cmd("Dart pub", "dart pub cache clean")
	}
	if has("gem") {
		cmd("Ruby gem", "gem cleanup")
	}
	if has("go") {
		cmd("Go build cache", "go clean -cache")
	}
	if has("go") {
		cmd("Go module cache", "go clean -modcache")
	}
	if has("docker") {
		cmd("Docker builder", "docker builder prune -f")
	}
	if has("docker") {
		cmd("Docker unused images", "docker image prune -f")
	}
	if has("podman") {
		cmd("Podman system", "podman system prune -f")
	}
	return cmds
}

// Scan computes sizes for every target, applying the options filters.
func Scan(opts Options) []Target {
	var out []Target
	for _, tg := range Targets() {
		if !inCats(opts, tg.Category) || tg.Path == "" || !tg.Exists {
			continue
		}
		mb := dirSizeMB(tg.Path)
		if opts.MinSizeMB > 0 && mb < float64(opts.MinSizeMB) {
			continue
		}
		if opts.OlderThanDays > 0 && ageDays(tg.Path) < opts.OlderThanDays {
			continue
		}
		tg.SizeMB = mb
		out = append(out, tg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SizeMB > out[j].SizeMB })
	return out
}

// remove attempts to delete a path. Windows file handles that are still open
// cause ACCESS_DENIED; we retry once after a short wait.
func remove(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	time.Sleep(300 * time.Millisecond)
	return os.RemoveAll(path)
}

// Run performs a cleanup. If opts.DryRun, nothing is deleted.
func Run(opts Options) RunReport {
	var report RunReport
	for _, tg := range Scan(opts) {
		start := time.Now()
		r := Result{Target: tg}
		mb := tg.SizeMB

		if opts.DryRun {
			r.Status = "dry-run"
			r.RemovedMB = mb
			report.FreedMB += mb
		} else if err := remove(tg.Path); err != nil {
			r.Status = "locked"
			r.Error = err.Error()
			report.SkippedMB += mb
		} else if _, err := os.Stat(tg.Path); err == nil {
			r.Status = "locked"
			report.SkippedMB += mb
		} else {
			r.Status = "removed"
			r.RemovedMB = mb
			report.FreedMB += mb
		}
		r.ElapsedMs = time.Since(start).Milliseconds()
		report.Results = append(report.Results, r)
	}
	report.TotalMB = report.FreedMB + report.SkippedMB
	return report
}

// PurgeResult is the outcome of one tool-managed purge command.
type PurgeResult struct {
	Label     string `json:"label"`
	Command   string `json:"command"`
	Status    string `json:"status"` // ok | timeout | failed | dry-run
	ElapsedMs int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`
}

// Purge runs tool-managed cache purge commands with a per-command timeout so
// hung tools (yarn, docker, conda) cannot block the whole run.
func Purge(ctx context.Context, opts Options) []PurgeResult {
	timeoutSec := opts.PurgeTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	var (
		mu   sync.Mutex
		res  []PurgeResult
		wg   sync.WaitGroup
		cmds = PurgeCommands()
	)
	for _, c := range cmds {
		if !inCats(opts, c.Category) {
			continue
		}
		wg.Add(1)
		go func(c PurgeCmd) {
			defer wg.Done()
			start := time.Now()
			r := PurgeResult{Label: c.Label, Command: c.Command}
			if opts.DryRun {
				r.Status = "dry-run"
				res = append(res, r)
				return
			}
			ctxT, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctxT, "cmd", "/c", c.Command)
			cmd.Stdout = nil
			cmd.Stderr = nil
			err := cmd.Run()
			r.ElapsedMs = time.Since(start).Milliseconds()
			if ctxT.Err() == context.DeadlineExceeded {
				r.Status = "timeout"
			} else if err != nil {
				r.Status = "failed"
				r.Error = err.Error()
			} else {
				r.Status = "ok"
			}
			mu.Lock()
			res = append(res, r)
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	sort.Slice(res, func(i, j int) bool { return res[i].Label < res[j].Label })
	return res
}

// Summary renders a short human-readable summary string.
func (r RunReport) Summary() string {
	return fmt.Sprintf("freed %.0f MB, skipped %.0f MB (locked/filtered), %d items",
		r.FreedMB, r.SkippedMB, len(r.Results))
}
