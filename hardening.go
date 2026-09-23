package main

// hardening.go — small shared guards for the local API and for helper
// subprocesses: the loopback Host rule, the navigation-site rule, config
// redaction, the minimal helper environment, and cleaning of tool output before
// it is stored or returned.

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ---- local API -------------------------------------------------------------

// loopbackHostAllowed reports whether r may reach /api given the Host header it
// carries. When the connection arrived on a loopback address, the Host must name
// loopback (127.0.0.1, localhost or [::1]) on the port the connection arrived
// on; any other Host is refused. Connections that arrived on a non-loopback
// address (a deliberate non-localhost bind, which requires a bearer token) are
// not restricted here.
func loopbackHostAllowed(r *http.Request) bool {
	la, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || la == nil {
		return false
	}
	lhost, lport, err := net.SplitHostPort(la.String())
	if err != nil {
		return false
	}
	if ip := net.ParseIP(lhost); ip == nil || !ip.IsLoopback() {
		return true
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		// No port in the Host header: only the default HTTP port omits it.
		host, port = r.Host, "80"
	}
	if port != lport {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	return host == "127.0.0.1" || host == "::1"
}

// navigationSiteAllowed applies the Fetch Metadata rule to the new-tab
// navigation exception: when the browser sends Sec-Fetch-Site, it must be
// same-origin or none (typed address, bookmark). Clients that send no
// Sec-Fetch-Site keep the existing Origin/Referer rule.
func navigationSiteAllowed(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
		return true
	}
	return false
}

// redactConfig returns cfg with secrets blanked for display in API responses.
// The stored configuration is unchanged.
func redactConfig(cfg Config) Config {
	cfg.AuthToken = ""
	return cfg
}

// ---- helper environment ------------------------------------------------------

// helperEnvKeys are the only parent environment variables a helper subprocess
// receives (matched case-insensitively on Windows). They cover temp folders,
// profile and home folders (gpg finds its home there), system folders, and
// locale. Everything else, including any OBELISK_* or MNEMO_* variable, is
// withheld.
var helperEnvKeys = []string{
	// Windows
	"SystemRoot", "SystemDrive", "windir", "ComSpec", "PATHEXT",
	"TEMP", "TMP", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
	"HOMEDRIVE", "HOMEPATH", "USERNAME", "ProgramData", "ProgramFiles",
	"ProgramFiles(x86)", "ProgramW6432", "CommonProgramFiles",
	"PROCESSOR_ARCHITECTURE", "NUMBER_OF_PROCESSORS", "PSModulePath",
	// POSIX
	"HOME", "USER", "LOGNAME", "TMPDIR", "TZ",
	// both
	"LANG", "LANGUAGE", "GNUPGHOME",
}

// withheldEnvPrefixes are never passed, even if a future edit adds a matching key
// to helperEnvKeys.
var withheldEnvPrefixes = []string{"OBELISK_", "MNEMO_"}

// helperEnv builds the environment for a helper subprocess. PATH holds the
// helper's own folder followed by the system folders, so a helper finds its
// companions (gpg finds gpg-agent) without inheriting the caller's PATH. With
// keepPath, PATH is the caller's PATH instead; the burn command uses this because
// it is a user-written command line that may call any installed program.
func helperEnv(toolPath string, keepPath bool) []string {
	allowed := map[string]bool{}
	for _, k := range helperEnvKeys {
		allowed[envKeyFold(k)] = true
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		if withheldEnvKey(k) {
			continue
		}
		fk := envKeyFold(k)
		if allowed[fk] || strings.HasPrefix(fk, envKeyFold("LC_")) {
			env = append(env, kv)
		}
	}
	path := os.Getenv("PATH")
	if !keepPath {
		path = helperPath(toolPath)
	}
	return append(env, "PATH="+path)
}

// helperPath is the PATH given to a helper: its own folder, then system folders.
func helperPath(toolPath string) string {
	var dirs []string
	if toolPath != "" && filepath.IsAbs(toolPath) {
		dirs = append(dirs, filepath.Dir(toolPath))
	}
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		dirs = append(dirs, filepath.Join(root, "System32"), root,
			filepath.Join(root, "System32", "WindowsPowerShell", "v1.0"))
	} else {
		dirs = append(dirs, "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

func withheldEnvKey(k string) bool {
	fk := envKeyFold(k)
	for _, p := range withheldEnvPrefixes {
		if strings.HasPrefix(fk, envKeyFold(p)) {
			return true
		}
	}
	return false
}

func envKeyFold(k string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(k)
	}
	return k
}

// helperCommand is exec.Command with the minimal helper environment.
func helperCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = helperEnv(cmd.Path, false)
	return cmd
}

// helperCommandContext is exec.CommandContext with the minimal helper environment.
func helperCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = helperEnv(cmd.Path, false)
	return cmd
}

// ---- stored tool text ----------------------------------------------------------

const (
	// toolOutputMax bounds the helper output kept in one error message.
	toolOutputMax = 700
	// storedErrorMax bounds any error text stored in a record or returned over HTTP.
	storedErrorMax = 2048
)

// cleanToolText makes helper output safe to store and display: invalid UTF-8
// and non-printable characters are dropped (newline and tab stay), carriage
// returns are removed, and the result is capped at max bytes on a character
// boundary with a visible marker.
func cleanToolText(s string, max int) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == utf8.RuneError && size <= 1 {
			continue
		}
		if r == '\n' || r == '\t' || unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if max > 0 && len(out) > max {
		const marker = " …[truncated]"
		cut := max - len(marker)
		if cut < 0 {
			cut = 0
		}
		for cut > 0 && !utf8.RuneStart(out[cut]) {
			cut--
		}
		out = out[:cut] + marker
	}
	return out
}

// toolOutputTail keeps the last toolOutputMax bytes of helper output (errors are
// usually at the end), cleaned.
func toolOutputTail(out []byte) string {
	if len(out) > toolOutputMax {
		out = out[len(out)-toolOutputMax:]
		for len(out) > 0 && !utf8.RuneStart(out[0]) {
			out = out[1:]
		}
	}
	return strings.TrimSpace(cleanToolText(string(out), toolOutputMax))
}

// storedErrorText is the form of an error message that may be stored in a
// catalog, burn queue or job record, or returned over HTTP.
func storedErrorText(s string) string {
	return cleanToolText(s, storedErrorMax)
}
