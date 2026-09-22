package tray

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func captureClipboard(test *testing.T) *[]string {
	test.Helper()
	var written []string
	previous := writeClipboard
	writeClipboard = func(text string) error {
		written = append(written, text)
		return nil
	}
	test.Cleanup(func() { writeClipboard = previous })
	return &written
}

func exportKindByLabel(test *testing.T, label string) exportKind {
	test.Helper()
	for _, kind := range exportKinds {
		if kind.label == label {
			return kind
		}
	}
	test.Fatalf("export kind %q missing from %+v", label, exportKinds)
	return exportKind{}
}

func stubCommands(test *testing.T, script string, names ...string) string {
	test.Helper()
	dir := test.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			test.Fatal(err)
		}
	}
	test.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestExportCommandUsesLiveLocalRule(test *testing.T) {
	// log.Error raises a desktop alert through these.
	stubCommands(test, "", "osascript", "notify-send")
	written := captureClipboard(test)
	app := &App{rules: []*ruleState{
		{name: "alpha", address: "127.0.0.1:1097", http: true},
		{name: "remote", address: "127.0.0.1:2097", remote: true, http: true},
		{name: "database", address: "127.0.0.1:15432", target: "10.0.0.5:5432"},
		{name: "wildcard", listenAddress: "0.0.0.0:1197", http: true},
		{name: "socks", address: "127.0.0.1:1081"},
	}}
	shell := exportKindByLabel(test, "Shell")
	for _, name := range []string{"remote", "database", "socks", "missing"} {
		app.exportCommand(shell, name)
	}
	if len(*written) != 0 {
		test.Fatalf("non-local or non-HTTP rule exported: %q", *written)
	}
	app.exportCommand(shell, "wildcard")
	app.exportCommand(shell, "alpha")
	app.mu.Lock()
	app.rules[0].address = "127.0.0.1:1199"
	app.mu.Unlock()
	app.exportCommand(shell, "alpha")
	want := []string{
		"export http_proxy=http://127.0.0.1:1197 https_proxy=http://127.0.0.1:1197; ",
		"export http_proxy=http://127.0.0.1:1097 https_proxy=http://127.0.0.1:1097; ",
		"export http_proxy=http://127.0.0.1:1199 https_proxy=http://127.0.0.1:1199; ",
	}
	if !reflect.DeepEqual(*written, want) {
		test.Fatalf("clipboard = %q, want %q", *written, want)
	}
	writeClipboard = func(string) error { return errors.New("clipboard unavailable") }
	app.exportCommand(shell, "alpha")
}

func TestExportCommandKinds(test *testing.T) {
	want := []struct{ label, command string }{
		{"Shell", "export http_proxy=http://127.0.0.1:1097 https_proxy=http://127.0.0.1:1097; "},
		{"Shell git", `export GIT_SSH_COMMAND='ssh -o ProxyCommand="nc -x 127.0.0.1:1097 %h %p"' http_proxy=http://127.0.0.1:1097 https_proxy=http://127.0.0.1:1097; `},
		{"Cmd", `set "http_proxy=http://127.0.0.1:1097" && set "https_proxy=http://127.0.0.1:1097"`},
		{"Cmd git", `set "GIT_SSH_COMMAND=ssh -o ProxyCommand='connect -S 127.0.0.1:1097 %h %p'" && set "http_proxy=http://127.0.0.1:1097" && set "https_proxy=http://127.0.0.1:1097"`},
		{"PowerShell", "$env:http_proxy='http://127.0.0.1:1097'; $env:https_proxy='http://127.0.0.1:1097'; "},
		{"PowerShell git", `$env:GIT_SSH_COMMAND='ssh -o ProxyCommand="connect -S 127.0.0.1:1097 %h %p"'; $env:http_proxy='http://127.0.0.1:1097'; $env:https_proxy='http://127.0.0.1:1097'; `},
	}
	if len(exportKinds) != len(want) {
		test.Fatalf("%d export kinds, want %d", len(exportKinds), len(want))
	}
	for index, kind := range exportKinds {
		if kind.label != want[index].label {
			test.Errorf("export kind %d = %q, want %q", index, kind.label, want[index].label)
		}
		if got := kind.command("127.0.0.1:1097"); got != want[index].command {
			test.Errorf("%s:\n got %q\nwant %q", kind.label, got, want[index].command)
		}
	}
}

// runGitSSH launches ssh the way git runs GIT_SSH_COMMAND and returns the argv the ProxyCommand received.
func runGitSSH(test *testing.T, gitSSHCommand, record string) []string {
	test.Helper()
	ssh := exec.Command("sh", "-c", gitSSHCommand+` "$@"`, "sh",
		"-F", "/dev/null", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5", "-p", "2222", "example.invalid")
	out, err := ssh.CombinedOutput()
	if err == nil {
		test.Fatalf("ssh succeeded without a proxy: %s", out)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		test.Fatalf("ssh never ran the ProxyCommand: %v: %s", err, out)
	}
	os.Remove(record)
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

func TestExportCommandGitProxyCommands(test *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil || runtime.GOOS == "windows" {
		test.Skip("needs sh and ssh")
	}
	const address = "127.0.0.1:1097"
	dir := stubCommands(test, "printf '%s\\0' \"$@\" > \"$(dirname \"$0\")/argv\"\nexit 1\n", "nc", "connect")
	record := filepath.Join(dir, "argv")
	shellGit := exportKindByLabel(test, "Shell git").command(address)
	shellGitSSH, err := exec.Command("sh", "-c", shellGit+`printf '%s' "$GIT_SSH_COMMAND"`).Output()
	if err != nil {
		test.Fatalf("sh: %v", err)
	}
	if got, want := runGitSSH(test, string(shellGitSSH), record), []string{"-x", address, "example.invalid", "2222"}; !reflect.DeepEqual(got, want) {
		test.Fatalf("nc argv = %q, want %q", got, want)
	}
	// Git for Windows hands GIT_SSH_COMMAND to its bundled sh.
	for label, gitSSHCommand := range map[string]string{
		"Cmd git":        `ssh -o ProxyCommand='connect -S 127.0.0.1:1097 %h %p'`,
		"PowerShell git": `ssh -o ProxyCommand="connect -S 127.0.0.1:1097 %h %p"`,
	} {
		if command := exportKindByLabel(test, label).command(address); !strings.Contains(command, gitSSHCommand) {
			test.Fatalf("%s = %q, want it to set GIT_SSH_COMMAND %q", label, command, gitSSHCommand)
		}
		if got, want := runGitSSH(test, gitSSHCommand, record), []string{"-S", address, "example.invalid", "2222"}; !reflect.DeepEqual(got, want) {
			test.Fatalf("%s connect argv = %q, want %q", label, got, want)
		}
	}
}
