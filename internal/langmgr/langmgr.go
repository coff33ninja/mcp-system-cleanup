package langmgr

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Language describes a supported language and how to detect/install it.
type Language struct {
	Name       string              `json:"name"`
	Commands   []string            `json:"commands"`    // binaries that indicate presence
	VersionArg []string            `json:"version_arg"` // e.g. ["--version"]
	Methods    []string            `json:"methods"`     // uv, winget, choco
	Install    map[string][]string `json:"-"`           // method -> commands
	Uninstall  map[string][]string `json:"-"`           // method -> commands
}

// Detection is the runtime result for a language.
type Detection struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Present bool   `json:"present"`
}

// All returns the language table, inspired by the local cleanup tools
// used to manage dev environments on this system.
func All() []Language {
	python := Language{
		Name: "Python", Commands: []string{"python", "py"}, VersionArg: []string{"--version"}, Methods: []string{"uv", "winget", "choco"},
		Install: map[string][]string{
			"uv":     {"uv python install 3.13"},
			"winget": {"winget install --id Python.Python.3.13 -e --silent", "winget install --id Python.Launcher -e --silent"},
			"choco":  {"choco install python313 -y"},
		},
		Uninstall: map[string][]string{
			"uv":     {"uv python uninstall 3.13"},
			"winget": {"winget uninstall --id Python.Python.3.13 --silent"},
			"choco":  {"choco uninstall python313 -y"},
		},
	}
	node := Language{
		Name: "Node.js", Commands: []string{"node"}, VersionArg: []string{"--version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id OpenJS.NodeJS.LTS -e --silent"},
			"choco":  {"choco install nodejs-lts -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id OpenJS.NodeJS.LTS --silent"},
			"choco":  {"choco uninstall nodejs-lts -y"},
		},
	}
	golang := Language{
		Name: "Go", Commands: []string{"go"}, VersionArg: []string{"version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id GoLang.Go -e --silent"},
			"choco":  {"choco install golang -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id GoLang.Go --silent"},
			"choco":  {"choco uninstall golang -y"},
		},
	}
	rust := Language{
		Name: "Rust", Commands: []string{"rustc"}, VersionArg: []string{"--version"}, Methods: []string{"winget"},
		Install: map[string][]string{
			"winget": {"winget install --id Rustlang.Rustup -e --silent"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id Rustlang.Rustup --silent"},
		},
	}
	zig := Language{
		Name: "Zig", Commands: []string{"zig"}, VersionArg: []string{"version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id zig.zig -e --silent"},
			"choco":  {"choco install zig -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id zig.zig --silent"},
			"choco":  {"choco uninstall zig -y"},
		},
	}
	java := Language{
		Name: "Java", Commands: []string{"java"}, VersionArg: []string{"-version"}, Methods: []string{"winget"},
		Install: map[string][]string{
			"winget": {"winget install --id EclipseAdoptium.Temurin.21.JDK -e --silent"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id EclipseAdoptium.Temurin.21.JDK --silent"},
		},
	}
	dotnet := Language{
		Name: ".NET SDK", Commands: []string{"dotnet"}, VersionArg: []string{"--version"}, Methods: []string{"winget"},
		Install: map[string][]string{
			"winget": {"winget install --id Microsoft.DotNet.SDK.9 -e --silent"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id Microsoft.DotNet.SDK.9 --silent"},
		},
	}
	dart := Language{
		Name: "Dart", Commands: []string{"dart"}, VersionArg: []string{"--version"}, Methods: []string{"winget"},
		Install: map[string][]string{
			"winget": {"winget install --id Dart.Dart -e --silent"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id Dart.Dart --silent"},
		},
	}
	flutter := Language{
		Name: "Flutter", Commands: []string{"flutter"}, VersionArg: []string{"--version"}, Methods: []string{"winget"},
		Install: map[string][]string{
			"winget": {"winget install --id Google.Flutter -e --silent"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id Google.Flutter --silent"},
		},
	}
	gcc := Language{
		Name: "GCC/MinGW", Commands: []string{"gcc"}, VersionArg: []string{"--version"}, Methods: []string{"choco"},
		Install: map[string][]string{
			"choco": {"choco install mingw -y"},
		},
		Uninstall: map[string][]string{
			"choco": {"choco uninstall mingw -y"},
		},
	}
	lua := Language{
		Name: "Lua", Commands: []string{"lua"}, VersionArg: []string{"-v"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id DEVCOM.Lua -e --silent"},
			"choco":  {"choco install lua -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id DEVCOM.Lua --silent"},
			"choco":  {"choco uninstall lua -y"},
		},
	}
	perl := Language{
		Name: "Perl", Commands: []string{"perl"}, VersionArg: []string{"--version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id StrawberryPerl.StrawberryPerl -e --silent"},
			"choco":  {"choco install strawberryperl -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id StrawberryPerl.StrawberryPerl --silent"},
			"choco":  {"choco uninstall strawberryperl -y"},
		},
	}
	r := Language{
		Name: "R", Commands: []string{"R"}, VersionArg: []string{"--version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id RProject.R -e --silent"},
			"choco":  {"choco install r.project -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id RProject.R --silent"},
			"choco":  {"choco uninstall r.project -y"},
		},
	}
	kotlin := Language{
		Name: "Kotlin", Commands: []string{"kotlinc"}, VersionArg: []string{"-version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id JetBrains.Kotlin.Compiler -e --silent"},
			"choco":  {"choco install kotlinc -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id JetBrains.Kotlin.Compiler --silent"},
			"choco":  {"choco uninstall kotlinc -y"},
		},
	}
	scala := Language{
		Name: "Scala", Commands: []string{"scala"}, VersionArg: []string{"-version"}, Methods: []string{"choco"},
		Install: map[string][]string{
			"choco": {"choco install scala -y"},
		},
		Uninstall: map[string][]string{
			"choco": {"choco uninstall scala -y"},
		},
	}
	elixir := Language{
		Name: "Elixir", Commands: []string{"elixir"}, VersionArg: []string{"--version"}, Methods: []string{"choco"},
		Install: map[string][]string{
			"choco": {"choco install elixir -y"},
		},
		Uninstall: map[string][]string{
			"choco": {"choco uninstall elixir -y"},
		},
	}
	erlang := Language{
		Name: "Erlang", Commands: []string{"erl"}, VersionArg: []string{"-version"}, Methods: []string{"choco"},
		Install: map[string][]string{
			"choco": {"choco install erlang -y"},
		},
		Uninstall: map[string][]string{
			"choco": {"choco uninstall erlang -y"},
		},
	}
	swift := Language{
		Name: "Swift", Commands: []string{"swift"}, VersionArg: []string{"--version"}, Methods: []string{"winget"},
		Install: map[string][]string{
			"winget": {"winget install --id Swift.Toolchain -e --silent"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id Swift.Toolchain --silent"},
		},
	}
	haskell := Language{
		Name: "Haskell (GHC)", Commands: []string{"ghc"}, VersionArg: []string{"--version"}, Methods: []string{"choco"},
		Install: map[string][]string{
			"choco": {"choco install ghc -y"},
		},
		Uninstall: map[string][]string{
			"choco": {"choco uninstall ghc -y"},
		},
	}
	llvm := Language{
		Name: "LLVM/Clang", Commands: []string{"clang"}, VersionArg: []string{"--version"}, Methods: []string{"winget", "choco"},
		Install: map[string][]string{
			"winget": {"winget install --id LLVM.LLVM -e --silent"},
			"choco":  {"choco install llvm -y"},
		},
		Uninstall: map[string][]string{
			"winget": {"winget uninstall --id LLVM.LLVM --silent"},
			"choco":  {"choco uninstall llvm -y"},
		},
	}
	ocaml := Language{
		Name: "OCaml", Commands: []string{"ocaml"}, VersionArg: []string{"-version"}, Methods: []string{"choco"},
		Install: map[string][]string{
			"choco": {"choco install ocaml -y"},
		},
		Uninstall: map[string][]string{
			"choco": {"choco uninstall ocaml -y"},
		},
	}

	return []Language{python, node, golang, rust, zig, java, dotnet, dart, flutter,
		gcc, lua, perl, r, kotlin, scala, elixir, erlang, swift, haskell, llvm, ocaml}
}

// Find returns a language by case-insensitive name.
func Find(name string) *Language {
	for i := range All() {
		if strings.EqualFold(All()[i].Name, name) {
			return &All()[i]
		}
	}
	return nil
}

// runVersion runs a binary to capture its version string (stderr fallback).
func runVersion(bin string, args []string) string {
	cmd := exec.Command(bin, args...)
	var sb strings.Builder
	cmd.Stdout = &sb
	cmd.Stderr = &sb
	_ = cmd.Run()
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "Installed"
	}
	if i := strings.Index(out, "\n"); i >= 0 {
		out = out[:i]
	}
	return out
}

// Detect scans for installed languages. Runs detection in parallel.
func Detect() []Detection {
	var (
		mu    sync.Mutex
		res   []Detection
		wg    sync.WaitGroup
		langs = All()
	)
	for i := range langs {
		wg.Add(1)
		go func(l Language) {
			defer wg.Done()
			d := Detection{Name: l.Name}
			for _, bin := range l.Commands {
				if p, err := exec.LookPath(bin); err == nil && p != "" {
					d.Present = true
					d.Version = runVersion(bin, l.VersionArg)
					break
				}
			}
			mu.Lock()
			res = append(res, d)
			mu.Unlock()
		}(langs[i])
	}
	wg.Wait()
	return res
}

// StepResult is the result of a single install/uninstall command.
type StepResult struct {
	Command   string `json:"command"`
	Status    string `json:"status"` // ok | failed
	ElapsedMs int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`
}

// Manage runs install or uninstall commands for a language/method with a
// per-command timeout.
func Manage(ctx context.Context, lang *Language, action, method string, timeoutSec int) []StepResult {
	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	table := lang.Install
	if action == "uninstall" {
		table = lang.Uninstall
	}
	cmds, ok := table[method]
	if !ok {
		return []StepResult{{Command: fmt.Sprintf("%s via %s", action, method), Status: "failed",
			Error: fmt.Sprintf("method %q not supported for %s", method, lang.Name)}}
	}
	var results []StepResult
	for _, c := range cmds {
		start := time.Now()
		ctxT, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		cmd := exec.CommandContext(ctxT, "cmd", "/c", c)
		out, err := cmd.CombinedOutput()
		cancel()
		r := StepResult{Command: c, ElapsedMs: time.Since(start).Milliseconds()}
		if err != nil {
			r.Status = "failed"
			r.Error = fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out)))
		} else {
			r.Status = "ok"
		}
		results = append(results, r)
	}
	return results
}
