//go:build !guionly

package main

// session_signin.go: how the owner gets a sign-in link.
//
// A running Obelisk with no token keeps a small file that only its OS account can
// read, outside the data folder so no app-state backup can pick it up:
//
//	Windows  %LocalAppData%\Obelisk\signin\port-<port>.json (ACL: this user only)
//	Linux    $XDG_RUNTIME_DIR/Obelisk/signin, else ~/.cache/Obelisk/signin (0700/0600)
//
// obelisk -signin reads it, proves it holds the key, and opens a new one-time link.
//
// On Linux and macOS, any account can read a process's command line, so the browser
// is pointed at an owner-only local HTML file that forwards to the link. On Windows,
// another standard account cannot read it, so the link is opened directly. That
// reaches the default browser even when .html files open in an editor.

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type signinRecord struct {
	Version int    `json:"version"`
	Addr    string `json:"addr"`
	PID     int    `json:"pid"`
	Key     string `json:"key"`
}

// signinDir is the per-user folder for sign-in files, created private if missing.
func signinDir() (string, error) {
	base := ""
	if runtime.GOOS != "windows" {
		base = os.Getenv("XDG_RUNTIME_DIR")
	}
	if base == "" {
		var err error
		if base, err = os.UserCacheDir(); err != nil {
			return "", err
		}
	}
	parent := filepath.Join(base, "Obelisk")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	dir := filepath.Join(parent, "signin")
	if err := ensurePrivateDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// signinFileName names the file for one exact listener address.
func signinFileName(addr string) string {
	safe := strings.Map(func(r rune) rune {
		if r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, addr)
	return "addr-" + safe + ".json"
}

// publishSigninFile writes this run's re-entry key for the same OS account.
func (s *sessionAuth) publishSigninFile() (string, error) {
	dir, err := signinDir()
	if err != nil {
		return "", err
	}
	removeStaleLaunchFiles(dir, time.Now())
	b, err := json.Marshal(signinRecord{Version: 2, Addr: s.addr, PID: os.Getpid(), Key: s.reentryKey()})
	if err != nil {
		return "", err
	}
	final := filepath.Join(dir, signinFileName(s.addr))
	f, err := createStage(final, 0o600)
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = renameReplacing(tmp, final)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return final, nil
}

// removeStaleLaunchFiles clears forwarding pages left by a run that ended early.
// Their codes expired long ago. Only files this code made are matched.
func removeStaleLaunchFiles(dir string, now time.Time) {
	matches, _ := filepath.Glob(filepath.Join(dir, "open-*.html"))
	for _, m := range matches {
		if fi, err := os.Lstat(m); err == nil && fi.Mode().IsRegular() && now.Sub(fi.ModTime()) > 2*signInLinkLifetime {
			_ = os.Remove(m)
		}
	}
}

func readSigninFile(addr string) (*signinRecord, error) {
	dir, err := signinDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, signinFileName(addr))
	if err := verifyPrivateFile(path); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec signinRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, fmt.Errorf("the sign-in file is not readable: %w", err)
	}
	if rec.Version != 2 || rec.Addr != addr || rec.Key == "" {
		return nil, errors.New("the sign-in file is not for this address")
	}
	return &rec, nil
}

// signInLaunchPage is the local page that forwards the browser to the one-time link.
func signInLaunchPage(link string) string {
	e := html.EscapeString(link)
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="referrer" content="no-referrer">` +
		`<meta http-equiv="refresh" content="0;url=` + e + `"><title>Opening Obelisk</title></head>` +
		`<body><p>Opening Obelisk. <a href="` + e + `">Continue</a></p></body></html>`
}

// openBrowserAt is replaced in tests so that no test opens a real browser.
var openBrowserAt = func(target string) error {
	argv := []string{"xdg-open", target}
	switch runtime.GOOS {
	case "windows":
		argv = []string{"rundll32", "url.dll,FileProtocolHandler", target}
	case "darwin":
		argv[0] = "open"
	}
	// This keeps the caller's PATH, where a desktop's browser may be (/snap/bin), and
	// withholds every Obelisk variable.
	cmd := burnProgramCommand(argv)
	cmd.Env = append(cmd.Env, desktopSessionEnv(os.Getenv)...)
	return cmd.Start()
}

// desktopSessionEnv is what a desktop opener needs to find the user's session.
func desktopSessionEnv(get func(string) string) []string {
	var env []string
	for _, k := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR", "XDG_DATA_DIRS",
		"XDG_CONFIG_DIRS", "XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE", "DESKTOP_SESSION", "DBUS_SESSION_BUS_ADDRESS"} {
		if v := get(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// openSignInLink opens link, through a private forwarding page where a command line
// is visible to other accounts. It returns that page's path, or "" if none was made.
func openSignInLink(link string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", openBrowserAt(link)
	}
	dir, err := signinDir()
	if err != nil {
		return "", err
	}
	f, err := createOwnerOnlyTemp(dir, "open-*.html")
	if err != nil {
		return "", err
	}
	page := f.Name()
	_, err = io.WriteString(f, signInLaunchPage(link))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		abs := filepath.ToSlash(page)
		if !strings.HasPrefix(abs, "/") {
			abs = "/" + abs
		}
		err = openBrowserAt((&url.URL{Scheme: "file", Path: abs}).String())
	}
	if err != nil {
		_ = os.Remove(page)
		return "", err
	}
	return page, nil
}

// startOwnerSignIn publishes the sign-in file and opens the owner's browser. Neither
// failing stops the server. Messages name no key, code or link.
func (s *sessionAuth) startOwnerSignIn(base string, logf func(string, ...any)) (cleanup func()) {
	file, err := s.publishSigninFile()
	if err != nil {
		logf("sign-in file could not be written, so obelisk -signin will not work for this run: %v", err)
	}
	cleanup = func() {
		if file != "" {
			_ = os.Remove(file)
		}
	}
	// A started opener can still fail to show anything, so the way in is always printed.
	recovery := "obelisk -signin -print-link -listen " + strings.TrimPrefix(base, "http://")
	if os.Getenv("OBELISK_NO_BROWSER") != "" {
		logf("not opening a browser. To sign in, run: %s", recovery)
		return cleanup
	}
	code, err := s.mintCode()
	if err == nil {
		var page string
		if page, err = openSignInLink(base + signInPath + "?code=" + url.QueryEscape(code)); err == nil && page != "" {
			time.AfterFunc(signInLinkLifetime, func() { _ = os.Remove(page) })
		}
	}
	if err != nil {
		logf("could not open a browser (%v). To sign in, run: %s", err, recovery)
	} else {
		logf("opening your browser. If it does not appear signed in, run: %s", recovery)
	}
	return cleanup
}

// runSignIn is obelisk -signin: it gets a new one-time link from the running server.
func runSignIn(addr string, printLink bool, stdout, stderr io.Writer) int {
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "obelisk: "+format+"\n", a...)
		return 1
	}
	if !isLocalhostAddr(addr) {
		return fail("-signin works only for an Obelisk on this computer (a loopback -listen address). A remote server uses its access token.")
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fail("cannot read the address %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return fail("-signin needs the port Obelisk is using, for example -listen 127.0.0.1:7821")
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	exact := net.JoinHostPort(host, portText)
	rec, err := readSigninFile(exact)
	if errors.Is(err, os.ErrNotExist) {
		return fail("no Obelisk started by this account is listening on %s. Start Obelisk, or add -listen with the address it uses.", exact)
	}
	if err != nil {
		return fail("cannot use the sign-in file: %v", err)
	}
	base := "http://" + exact
	code, err := requestReentryCode(base, exact, rec.Key)
	if err != nil {
		return fail("%v", err)
	}
	link := base + signInPath + "?code=" + url.QueryEscape(code)
	if printLink {
		fmt.Fprintf(stdout, "Open this link within %d seconds. It works once:\n%s\n", int(signInLinkLifetime/time.Second), link)
		return 0
	}
	page, err := openSignInLink(link)
	if err != nil {
		return fail("could not open a browser (%v). Run obelisk -signin -print-link and open the link yourself.", err)
	}
	fmt.Fprintln(stdout, "Opening Obelisk in your browser. If nothing appears, run obelisk -signin -print-link and open the link yourself.")
	if page != "" {
		// The browser reads the page just after it starts; the code expires regardless.
		time.Sleep(5 * time.Second)
		_ = os.Remove(page)
	}
	return 0
}

func requestReentryCode(base, addr, key string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(raw[:])
	body, _ := json.Marshal(map[string]string{"nonce": nonce, "proof": reentryClientProof(key, addr, nonce)})
	req, err := http.NewRequest(http.MethodPost, base+sessionReenterPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(requestedByHeader, requestedByValue)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("no Obelisk answered on %s. It may have stopped; start it again", addr)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the program on %s refused the sign-in (HTTP %d). It may be a different Obelisk run; restart Obelisk", addr, res.StatusCode)
	}
	var out struct {
		Sealed string `json:"sealed"`
		Proof  string `json:"proof"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&out); err != nil {
		return "", errors.New("the sign-in answer could not be read")
	}
	sealed, err := hex.DecodeString(out.Sealed)
	if err != nil || len(sealed) == 0 || !hmac.Equal([]byte(out.Proof), []byte(reentryServerProof(key, addr, nonce, out.Sealed))) {
		return "", fmt.Errorf("the program on %s could not prove it is this account's Obelisk, so no browser was opened", addr)
	}
	return string(sealReentryCode(key, addr, nonce, sealed)), nil
}
