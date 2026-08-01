package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/KPO-Tech/seshat/pkg/runtimepath"
)

type backgroundSessionStatus string

const (
	backgroundStatusRunning backgroundSessionStatus = "running"
	backgroundStatusExited  backgroundSessionStatus = "exited"
	backgroundStatusFailed  backgroundSessionStatus = "failed"
	backgroundStatusStale   backgroundSessionStatus = "stale"
	backgroundStatusKilled  backgroundSessionStatus = "killed"
	backgroundStatusUnknown backgroundSessionStatus = "unknown"
)

type backgroundSession struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name,omitempty"`
	PID       int                     `json:"pid"`
	Cwd       string                  `json:"cwd"`
	Status    backgroundSessionStatus `json:"status"`
	Provider  string                  `json:"provider,omitempty"`
	Model     string                  `json:"model,omitempty"`
	Prompt    string                  `json:"prompt,omitempty"`
	Command   []string                `json:"command"`
	StartedAt string                  `json:"started_at"`
	UpdatedAt string                  `json:"updated_at"`
	StdoutLog string                  `json:"stdout_log"`
	StderrLog string                  `json:"stderr_log"`
}

type backgroundRunConfig struct {
	Name           string
	Prompt         string
	Model          string
	PermissionMode string
	Cwd            string
	DBPath         string
	ShowThinking   bool
	Debug          bool
	Options        runtimeOptions
}

func backgroundRoot() string {
	return runtimepath.Join("", "bg-sessions")
}

func backgroundSessionsDir() string {
	return filepath.Join(backgroundRoot(), "sessions")
}

func backgroundLogsDir() string {
	return filepath.Join(backgroundRoot(), "logs")
}

func backgroundNamesDir() string {
	return filepath.Join(backgroundRoot(), "names")
}

func ensureBackgroundDirs() error {
	for _, dir := range []string{backgroundSessionsDir(), backgroundLogsDir(), backgroundNamesDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func backgroundLogPaths(id string) (string, string) {
	return filepath.Join(backgroundLogsDir(), id+".out.log"), filepath.Join(backgroundLogsDir(), id+".err.log")
}

func runBackgroundSession(cfg backgroundRunConfig, stdout io.Writer) error {
	if strings.TrimSpace(cfg.Prompt) == "" {
		return fmt.Errorf("missing prompt: use `seshat run --bg \"prompt\"`")
	}
	if err := ensureBackgroundNameAvailable(cfg.Name); err != nil {
		return err
	}
	if err := ensureBackgroundDirs(); err != nil {
		return err
	}

	id := "bg-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	stdoutLog, stderrLog := backgroundLogPaths(id)
	outFile, err := os.OpenFile(stdoutLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open stdout log: %w", err)
	}
	defer outFile.Close()
	errFile, err := os.OpenFile(stderrLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open stderr log: %w", err)
	}
	defer errFile.Close()

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	childArgs := []string{"run", "--background-child", "--background-id", id}
	if cfg.ShowThinking {
		childArgs = append(childArgs, "--show-thinking")
	}
	if cfg.Debug {
		childArgs = append(childArgs, "--debug")
	}
	childArgs = appendStringFlag(childArgs, "--model", cfg.Model)
	childArgs = appendStringFlag(childArgs, "--permission-mode", cfg.PermissionMode)
	childArgs = appendStringFlag(childArgs, "--cwd", cfg.Cwd)
	childArgs = appendStringFlag(childArgs, "--db", cfg.DBPath)
	childArgs = append(childArgs, cfg.Prompt)

	cmd := exec.Command(exe, childArgs...)
	cmd.Stdin = nil
	cmd.Stdout = outFile
	cmd.Stderr = errFile
	cmd.Dir = cfg.Options.WorkingDir
	cmd.Env = os.Environ()
	cmd.SysProcAttr = newBackgroundSysProcAttr()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background session: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	session := backgroundSession{
		ID:        id,
		Name:      strings.TrimSpace(cfg.Name),
		PID:       cmd.Process.Pid,
		Cwd:       cfg.Options.WorkingDir,
		Status:    backgroundStatusRunning,
		Provider:  string(cfg.Options.Model.Provider),
		Model:     cfg.Options.Model.Model,
		Prompt:    cfg.Prompt,
		Command:   append([]string{exe}, childArgs...),
		StartedAt: now,
		UpdatedAt: now,
		StdoutLog: stdoutLog,
		StderrLog: stderrLog,
	}
	if err := saveBackgroundSession(session); err != nil {
		_ = killProcessTree(cmd.Process.Pid)
		return err
	}
	if session.Name != "" {
		if err := reserveBackgroundName(session.Name, session.ID); err != nil {
			_ = killProcessTree(cmd.Process.Pid)
			_, _ = markBackgroundSessionStatus(session.ID, backgroundStatusKilled)
			return err
		}
	}
	_ = cmd.Process.Release()

	fmt.Fprintf(stdout, "Started background session %s", session.ID)
	if session.Name != "" {
		fmt.Fprintf(stdout, " (%s)", session.Name)
	}
	fmt.Fprintf(stdout, ".\nLogs: %s\n", session.StdoutLog)
	return nil
}

func appendStringFlag(args []string, name, value string) []string {
	if strings.TrimSpace(value) == "" {
		return args
	}
	return append(args, name, value)
}

func runBackgroundPS(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ps", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	sessions, err := refreshBackgroundSessions()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Fprintln(stdout, "No background sessions.")
		return nil
	}
	fmt.Fprintln(stdout, "ID               NAME             PID      STATUS   UPDATED              MODEL")
	for _, s := range sessions {
		fmt.Fprintf(stdout, "%-16s %-16s %-8d %-8s %-20s %s\n",
			s.ID, emptyDash(s.Name), s.PID, s.Status, shortTime(s.UpdatedAt), modelDisplay(s))
	}
	return nil
}

func runBackgroundLogs(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("logs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	follow := flags.Bool("f", false, "")
	stream := flags.String("stream", "stdout", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: seshat logs <id-or-name> [-f] [--stream stdout|stderr]")
	}
	session, err := resolveBackgroundSession(flags.Arg(0))
	if err != nil {
		return err
	}
	path := session.StdoutLog
	if *stream == "stderr" {
		path = session.StderrLog
	} else if *stream != "stdout" {
		return fmt.Errorf("--stream must be stdout or stderr")
	}
	return streamBackgroundLog(path, session.ID, *follow, stdout)
}

func runBackgroundKill(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("kill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: seshat kill <id-or-name>")
	}
	session, err := resolveBackgroundSession(flags.Arg(0))
	if err != nil {
		return err
	}
	if isTerminalBackgroundStatus(session.Status) {
		_, _ = markBackgroundSessionStatus(session.ID, backgroundStatusKilled)
		fmt.Fprintf(stdout, "Background session %s is already %s.\n", session.ID, session.Status)
		return nil
	}
	if alive, _ := isProcessAlive(session.PID); alive {
		if err := killProcessTree(session.PID); err != nil {
			return fmt.Errorf("kill background session %s: %w", session.ID, err)
		}
	}
	updated, err := markBackgroundSessionStatus(session.ID, backgroundStatusKilled)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Killed background session %s.\n", updated.ID)
	return nil
}

func runBackgroundAttach(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("attach", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: seshat attach <id-or-name>")
	}
	session, err := resolveBackgroundSession(flags.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Attach is not implemented for local background sessions yet. Use `seshat logs %s -f` to follow output.\n", session.ID)
	return nil
}

func saveBackgroundSession(session backgroundSession) error {
	if err := ensureBackgroundDirs(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(backgroundSessionsDir(), session.ID+"."+strconv.Itoa(os.Getpid())+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, backgroundMetadataPath(session.ID))
}

func loadBackgroundSession(id string) (backgroundSession, error) {
	data, err := os.ReadFile(backgroundMetadataPath(id))
	if err != nil {
		return backgroundSession{}, err
	}
	var session backgroundSession
	if err := json.Unmarshal(data, &session); err != nil {
		return backgroundSession{}, err
	}
	if session.ID == "" {
		session.ID = id
	}
	return session, nil
}

func listBackgroundSessions() ([]backgroundSession, error) {
	if err := ensureBackgroundDirs(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(backgroundSessionsDir())
	if err != nil {
		return nil, err
	}
	var sessions []backgroundSession
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		session, err := loadBackgroundSession(strings.TrimSuffix(entry.Name(), ".json"))
		if err == nil {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})
	return sessions, nil
}

func refreshBackgroundSessions() ([]backgroundSession, error) {
	sessions, err := listBackgroundSessions()
	if err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].Status != backgroundStatusRunning && sessions[i].Status != backgroundStatusUnknown {
			continue
		}
		alive, err := isProcessAlive(sessions[i].PID)
		if err == nil && alive {
			continue
		}
		sessions[i].Status = backgroundStatusStale
		sessions[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_ = saveBackgroundSession(sessions[i])
	}
	return sessions, nil
}

func resolveBackgroundSession(target string) (backgroundSession, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return backgroundSession{}, fmt.Errorf("missing background session id or name")
	}
	if session, err := loadBackgroundSession(target); err == nil {
		return session, nil
	}
	sessions, err := listBackgroundSessions()
	if err != nil {
		return backgroundSession{}, err
	}
	var matches []backgroundSession
	for _, s := range sessions {
		if s.Name == target || strings.HasPrefix(s.ID, target) {
			matches = append(matches, s)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return backgroundSession{}, fmt.Errorf("ambiguous background session %q", target)
	}
	return backgroundSession{}, fmt.Errorf("no background session found for %q", target)
}

func markBackgroundSessionStatus(id string, status backgroundSessionStatus) (backgroundSession, error) {
	session, err := loadBackgroundSession(id)
	if err != nil {
		return backgroundSession{}, err
	}
	session.Status = status
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveBackgroundSession(session); err != nil {
		return backgroundSession{}, err
	}
	if session.Name != "" && isTerminalBackgroundStatus(status) {
		_ = os.Remove(backgroundNamePath(session.Name))
	}
	return session, nil
}

func ensureBackgroundNameAvailable(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	sessions, err := refreshBackgroundSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if s.Name == name && !isTerminalBackgroundStatus(s.Status) {
			return fmt.Errorf("background session name %q is already in use", name)
		}
	}
	return nil
}

func reserveBackgroundName(name, id string) error {
	data, err := json.MarshalIndent(map[string]string{"name": name, "id": id}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(backgroundNamePath(name), data, 0o600)
}

func backgroundMetadataPath(id string) string {
	return filepath.Join(backgroundSessionsDir(), filepath.Base(id)+".json")
}

func backgroundNamePath(name string) string {
	sum := sha256.Sum256([]byte(name))
	return filepath.Join(backgroundNamesDir(), hex.EncodeToString(sum[:])+".json")
}

func streamBackgroundLog(path, id string, follow bool, stdout io.Writer) error {
	var position int64
	for {
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && follow {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
		if _, err := file.Seek(position, io.SeekStart); err != nil {
			_ = file.Close()
			return err
		}
		written, copyErr := io.Copy(stdout, file)
		position += written
		_ = file.Close()
		if copyErr != nil {
			return copyErr
		}
		if !follow {
			return nil
		}
		session, err := resolveBackgroundSession(id)
		if err == nil && isTerminalBackgroundStatus(session.Status) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func isTerminalBackgroundStatus(status backgroundSessionStatus) bool {
	return status == backgroundStatusExited || status == backgroundStatusFailed || status == backgroundStatusStale || status == backgroundStatusKilled
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func shortTime(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Local().Format("2006-01-02 15:04")
}

func modelDisplay(s backgroundSession) string {
	if s.Provider == "" && s.Model == "" {
		return "-"
	}
	if s.Provider == "" {
		return s.Model
	}
	if s.Model == "" {
		return s.Provider
	}
	return s.Provider + ":" + s.Model
}
