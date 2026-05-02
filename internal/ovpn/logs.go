package ovpn

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"
)

// LogEntry is a structured journal line for the Logs page.
type LogEntry struct {
	Timestamp time.Time `json:"ts"`
	Level     string    `json:"level"` // info | warn | err | ok
	Message   string    `json:"msg"`
}

// TailJournal returns the last n entries from the openvpn-server unit
// journal, classified into the SPA's level buckets.
func (m *Manager) TailJournal(ctx context.Context, n int) ([]LogEntry, error) {
	if n <= 0 {
		n = 200
	}
	cmd := exec.CommandContext(ctx, "journalctl",
		"-u", m.ServiceUnit,
		"-n", itoa(n),
		"--no-pager",
		"--output=short-iso",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var out []LogEntry
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "-- ") {
			continue
		}
		out = append(out, parseJournalLine(line))
	}
	_ = cmd.Wait()
	return out, sc.Err()
}

// parseJournalLine splits a `short-iso` journalctl line into ts / level / msg.
// Format: "2026-05-01T09:56:45+0800 host openvpn[3097]: chris/.. message"
func parseJournalLine(line string) LogEntry {
	e := LogEntry{Level: "info", Message: line}
	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return e
	}
	if t, err := time.Parse("2006-01-02T15:04:05-0700", parts[0]); err == nil {
		e.Timestamp = t
	}
	e.Message = parts[3]
	low := strings.ToLower(e.Message)
	switch {
	case strings.Contains(low, "auth_failed"), strings.Contains(low, "tls error"),
		strings.Contains(low, "verify error"), strings.Contains(low, "fatal"),
		strings.Contains(low, "error:"):
		e.Level = "err"
	case strings.Contains(low, "warning"), strings.Contains(low, "expir"):
		e.Level = "warn"
	case strings.Contains(low, "peer connection initiated"),
		strings.Contains(low, "tls: soft reset"),
		strings.Contains(low, "initialization sequence completed"):
		e.Level = "ok"
	}
	return e
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}
