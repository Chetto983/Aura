// This lab-only harness opens synthetic pages in an already-created browser box.
// It uses the production MCP SDK over SSH, without an LLM or a public tool-call API.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type options struct {
	ssh, host, user, identity, fixture, session, mode string
	sshPasswordEnv, sudoPasswordEnv                   string
	legacyShell                                       bool
	googleBaseline                                    bool
}

var sessionPattern = regexp.MustCompile(`^e2e-[A-Za-z0-9_-]{1,44}$`)
var userPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func (o options) validate() error {
	if id, err := uuid.Parse(o.identity); err != nil || id.String() != o.identity {
		return errors.New("--identity must be the dedicated member UUID")
	}
	if o.mode != "baseline" && !sessionPattern.MatchString(o.session) {
		return errors.New("--session must start e2e- and contain 5-48 safe characters")
	}
	if !userPattern.MatchString(o.user) || o.host == "" || strings.ContainsAny(o.host, " \t\r\n'\";/$`") {
		return errors.New("--host and --user must identify the authorized SSH target")
	}
	if o.mode != "open" && o.mode != "close" && o.mode != "baseline" {
		return errors.New("--mode must be open, close or baseline")
	}
	if o.legacyShell && o.mode != "baseline" {
		return errors.New("--legacy-shell is restricted to read-only baseline inventory")
	}
	if o.googleBaseline && (o.mode != "baseline" || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`).MatchString(o.session)) {
		return errors.New("--google-baseline requires baseline mode and one existing safe session name")
	}
	if o.mode == "open" {
		u, err := url.Parse(o.fixture)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
			return errors.New("--fixture must be a public HTTP(S) synthetic fixture URL")
		}
		host := strings.ToLower(u.Hostname())
		ip := net.ParseIP(host)
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast())) {
			return errors.New("--fixture must not address loopback or a private network")
		}
	}
	return nil
}

func main() {
	var o options
	flag.StringVar(&o.ssh, "ssh", "ssh", "SSH executable; ssh key/agent auth, or plink with password env")
	flag.StringVar(&o.host, "host", "", "authorized lab SSH hostname/IP")
	flag.StringVar(&o.user, "user", "aura", "SSH username")
	flag.StringVar(&o.identity, "identity", "", "dedicated member identity UUID; box must already exist")
	flag.StringVar(&o.fixture, "fixture", "", "public synthetic fixture URL (open mode only)")
	flag.StringVar(&o.session, "session", "", "unique e2e- browser session, maximum 48 characters")
	flag.StringVar(&o.mode, "mode", "open", "open synthetic fixture, close own e2e session, or baseline active-session metadata")
	flag.BoolVar(&o.legacyShell, "legacy-shell", false, "baseline only: inspect the existing pre-migration shell box instead of browser box")
	flag.BoolVar(&o.googleBaseline, "google-baseline", false, "authorized baseline only: reopen YouTube in the one existing --session profile; report only signed-in boolean")
	flag.StringVar(&o.sshPasswordEnv, "ssh-password-env", "AURA_E2E_SSH_PASSWORD", "password env name for plink; value is never printed")
	flag.StringVar(&o.sudoPasswordEnv, "sudo-password-env", "AURA_E2E_SUDO_PASSWORD", "sudo password env name; absent uses sudo -n")
	flag.Parse()
	if err := o.validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(o options) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	password := os.Getenv(o.sudoPasswordEnv)
	if strings.ContainsAny(password, "\r\n") {
		return errors.New("sudo password must not contain a newline")
	}
	sudo := "sudo -n "
	if password != "" {
		sudo = "sudo -S -p '' "
	}
	prefix := "aura-browser-"
	if o.legacyShell {
		prefix = "aura-box-"
	}
	remote := sudo + "docker exec -i " + prefix + o.identity + " agent-browser mcp"
	if o.mode == "baseline" {
		remote += " --tools all"
	}
	args := []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", o.user + "@" + o.host, remote}
	executable := o.ssh
	var sshPassword string
	if strings.Contains(strings.ToLower(filepath.Base(o.ssh)), "plink") {
		args = []string{"-batch", "-ssh", o.user + "@" + o.host}
		if secret := os.Getenv(o.sshPasswordEnv); secret != "" {
			file, err := os.CreateTemp("", "aura-e2e-ssh-*")
			if err != nil {
				return errors.New("cannot create private SSH password file")
			}
			defer func() { _ = os.Remove(file.Name()) }()
			_, writeErr := file.WriteString(secret)
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				return errors.New("cannot prepare private SSH password file")
			}
			args = append(args, "-pwfile", file.Name())
		}
		args = append(args, remote)
	} else if secret := os.Getenv(o.sshPasswordEnv); secret != "" {
		if strings.ContainsAny(secret, "\r\n") {
			return errors.New("SSH password must not contain a newline")
		}
		var err error
		executable, err = exec.LookPath("sshpass")
		if err != nil {
			return errors.New("password SSH requires installed sshpass")
		}
		args[1] = "BatchMode=no"
		args = append([]string{"-e", o.ssh}, args...)
		sshPassword = secret
	}
	command := exec.CommandContext(ctx, executable, args...) // #nosec G204 -- operator-chosen ssh/plink/sshpass, no shell; host, user and identity are validated
	if sshPassword != "" {
		command.Env = append(os.Environ(), "SSHPASS="+sshPassword)
	}
	command.Stderr = io.Discard
	input, err := command.StdinPipe()
	if err != nil {
		return errors.New("cannot prepare SSH stdin")
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return errors.New("cannot prepare SSH stdout")
	}
	if err := command.Start(); err != nil {
		return errors.New("cannot start SSH executable")
	}
	defer func() {
		_ = input.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	// sudo consumes one password line before docker exec receives the SDK's MCP stream.
	if password != "" {
		if _, err := io.WriteString(input, password+"\n"); err != nil {
			return errors.New("cannot supply sudo authentication")
		}
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "aura-browser-sidecar-e2e", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.IOTransport{Reader: output, Writer: input}, nil)
	if err != nil {
		return errors.New("MCP initialize failed; verify SSH access and existing browser box")
	}
	defer func() { _ = session.Close() }()
	if o.mode == "baseline" {
		if o.googleBaseline {
			return googleBaseline(ctx, session, o.session)
		}
		return baseline(ctx, session)
	}
	manifest, err := session.ListTools(ctx, nil)
	if err != nil || len(manifest.Tools) != 29 || manifest.NextCursor != "" {
		return errors.New("MCP inventory differs from pinned 29-tool core profile")
	}
	argsMap := map[string]any{"session": o.session, "extraArgs": []string{"--profile", "~/profiles/" + o.session}}
	if o.mode == "close" {
		return call(ctx, session, "agent_browser_close", argsMap)
	}
	argsMap["url"] = o.fixture
	if err := call(ctx, session, "agent_browser_open", argsMap); err != nil {
		return err
	}
	delete(argsMap, "url")
	if err := call(ctx, session, "agent_browser_snapshot", argsMap); err != nil {
		return err
	}
	return call(ctx, session, "agent_browser_get_title", argsMap)
}

func googleBaseline(ctx context.Context, client *sdkmcp.ClientSession, name string) error {
	args := map[string]any{"session": name, "extraArgs": []string{"--profile", "~/profiles/" + name}, "url": "https://www.youtube.com/"}
	result, err := client.CallTool(ctx, &sdkmcp.CallToolParams{Name: "agent_browser_open", Arguments: args})
	if err != nil || result == nil || result.IsError {
		return errors.New("baseline navigation to Google failed (content withheld)")
	}
	delete(args, "url")
	args["script"] = `({signed_in: location.hostname === 'www.youtube.com' && !!document.querySelector('ytd-masthead #avatar-btn')})`
	result, err = client.CallTool(ctx, &sdkmcp.CallToolParams{Name: "agent_browser_eval", Arguments: args})
	if err != nil || result == nil || result.IsError {
		return errors.New("baseline DOM check on Google failed (content withheld)")
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return errors.New("cannot decode baseline boolean")
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("cannot decode baseline boolean")
	}
	var signedIn *bool
	var find func(any)
	find = func(value any) {
		if object, ok := value.(map[string]any); ok {
			for key, child := range object {
				switch key {
				case "signed_in":
					if flag, ok := child.(bool); ok {
						signedIn = &flag
					}
				case "response", "data", "result":
					find(child)
				}
			}
		}
	}
	find(value)
	if signedIn == nil {
		return errors.New("baseline DOM did not return a boolean (content withheld)")
	}
	encoded, _ := json.Marshal(map[string]any{"session": name, "signed_in": *signedIn})
	fmt.Println(string(encoded))
	return nil
}

func baseline(ctx context.Context, session *sdkmcp.ClientSession) error {
	profiles, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "agent_browser_tools_profiles", Arguments: map[string]any{}})
	if err != nil || profiles == nil || profiles.IsError {
		return errors.New("MCP tool profile discovery failed (content withheld)")
	}
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "agent_browser_session_list", Arguments: map[string]any{}})
	if err != nil || result == nil || result.IsError {
		return errors.New("active-session inventory failed (content withheld)")
	}
	// Output only validated session names, never page/account content or tool dumps.
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return errors.New("cannot decode session metadata")
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("cannot decode session metadata")
	}
	names := []string{}
	entries := 0
	seen := false
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)
	var collect func(any)
	collect = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				switch key {
				case "sessions":
					if list, ok := child.([]any); ok {
						seen = true
						entries = len(list)
						for _, item := range list {
							name, _ := item.(string)
							if object, ok := item.(map[string]any); ok {
								name, _ = object["name"].(string)
							}
							if valid.MatchString(name) {
								names = append(names, name)
							}
						}
					}
				case "data", "response":
					collect(child)
				}
			}
		}
	}
	collect(value)
	encoded, _ := json.Marshal(map[string]any{"active_sessions": names, "session_entries": entries, "sessions_field_seen": seen})
	fmt.Println(string(encoded))
	return nil
}

func call(ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) error {
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || result == nil || result.IsError {
		return fmt.Errorf("%s failed (tool content withheld)", name)
	}
	if name == "agent_browser_get_title" {
		found := false
		for _, block := range result.Content {
			if text, ok := block.(*sdkmcp.TextContent); ok && strings.Contains(text.Text, "Browser sidecar fixture") {
				found = true
			}
		}
		if !found {
			return errors.New("synthetic fixture title was not observed (tool content withheld)")
		}
	}
	fmt.Printf("PASS %s content_blocks=%d\n", name, len(result.Content))
	return nil
}
