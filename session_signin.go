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
// The browser is pointed at an owner-only local HTML file that forwards to the link,
// never at the link itself: a process's command line can be visible to other
// accounts (on Linux, in /proc), and the link must not be.

import (
	"bytes"
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
	Port    int    `json:"port"`
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

func signinFileName(port int) string { return "port-" + strconv.Itoa(port) + ".json" }

// publishSigninFile writes this run's re-entry key for the same OS account.
func (s *sessionAuth) publishSigninFile() (string, error) {
	dir, err := signinDir()
	if err != nil {
		return "", err
	}
	removeStaleLaunchFiles(dir, time.Now())
	b, err := json.Marshal(signinRecord{Version: 1, Port: s.port, PID: os.Getpid(), Key: s.reentryKey()})
	if err != nil {
		return "", err
	}
	final := filepath.Join(dir, signinFileName(s.port))
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

func readSigninFile(port int) (*signinRecord, error) {
	dir, err := signinDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, signinFileName(port))
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
	if rec.Version != 1 || rec.Port != port || rec.Key == "" {
		return nil, errors.New("the sign-in file is not for this port")
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
	name, args := "xdg-open", []string{target}
	switch runtime.GOOS {
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", target}
	case "darwin":
		name = "open"
	}
	// The helper environment withholds every Obelisk variable. A desktop opener
	// also needs to find the user's display session.
	cmd := helperCommand(name, args...)
	for _, k := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
		if v := os.Getenv(k); v != "" {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	return cmd.Start()
}

// openSignInLink opens link through a private forwarding page and returns the page's path.
func openSignInLink(link string) (string, error) {
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
	if os.Getenv("OBELISK_NO_BROWSER") != "" {
		logf("not opening a browser. To sign in, run: obelisk -signin -listen %s", strings.TrimPrefix(base, "http://"))
		return cleanup
	}
	code, err := s.mintCode()
	if err == nil {
		var page string
		if page, err = openSignInLink(base + signInPath + "?code=" + url.QueryEscape(code)); err == nil {
			time.AfterFunc(signInLinkLifetime, func() { _ = os.Remove(page) })
		}
	}
	if err != nil {
		logf("could not open a browser (%v). To sign in, run: obelisk -signin -print-link -listen %s", err, strings.TrimPrefix(base, "http://"))
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
	rec, err := readSigninFile(port)
	if errors.Is(err, os.ErrNotExist) {
		return fail("no Obelisk started by this account is signed up on port %d. Start Obelisk, or add -listen with the port it uses.", port)
	}
	if err != nil {
		return fail("cannot use the sign-in file: %v", err)
	}
	base := "http://" + net.JoinHostPort(host, portText)
	code, err := requestReentryCode(base, portText, rec.Key)
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
	fmt.Fprintln(stdout, "Opening Obelisk in your browser.")
	// The browser reads the page just after it starts; the code expires regardless.
	time.Sleep(5 * time.Second)
	_ = os.Remove(page)
	return 0
}

func requestReentryCode(base, port, key string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(raw[:])
	body, _ := json.Marshal(map[string]string{"nonce": nonce, "proof": reentryClientProof(key, port, nonce)})
	req, err := http.NewRequest(http.MethodPost, base+sessionReenterPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(requestedByHeader, requestedByValue)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("no Obelisk answered on port %s. It may have stopped; start it again", port)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the program on port %s refused the sign-in (HTTP %d). It may be a different Obelisk run; restart Obelisk", port, res.StatusCode)
	}
	var out struct {
		Code  string `json:"code"`
		Proof string `json:"proof"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&out); err != nil {
		return "", errors.New("the sign-in answer could not be read")
	}
	if out.Code == "" || out.Proof != reentryServerProof(key, port, nonce, out.Code) {
		return "", fmt.Errorf("the program on port %s could not prove it is this account's Obelisk, so no browser was opened", port)
	}
	return out.Code, nil
}
