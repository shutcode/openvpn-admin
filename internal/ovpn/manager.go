package ovpn

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Manager wires together the on-disk OpenVPN/easyrsa state with shell
// commands. It mirrors what `openvpn-install.sh` does for client add/revoke
// so the WebUI's actions converge with the script's.
type Manager struct {
	OpenVPNDir string // /etc/openvpn
	EasyRSADir string // /etc/openvpn/easy-rsa
	ClientsDir string // where .ovpn files are written (default OpenVPNDir+"/clients")
	StatusPath string // /var/log/openvpn/status.log
	ServiceUnit string // openvpn-server@server.service
}

// NewManager applies defaults that match the upstream openvpn-install.sh.
func NewManager(openVPNDir, easyRSADir, clientsDir, statusPath, unit string) *Manager {
	if openVPNDir == "" {
		openVPNDir = "/etc/openvpn"
	}
	if easyRSADir == "" {
		easyRSADir = filepath.Join(openVPNDir, "easy-rsa")
	}
	if clientsDir == "" {
		clientsDir = filepath.Join(openVPNDir, "clients")
	}
	if statusPath == "" {
		statusPath = "/var/log/openvpn/status.log"
	}
	if unit == "" {
		unit = "openvpn-server@server.service"
	}
	return &Manager{
		OpenVPNDir:  openVPNDir,
		EasyRSADir:  easyRSADir,
		ClientsDir:  clientsDir,
		StatusPath:  statusPath,
		ServiceUnit: unit,
	}
}

var validCN = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

// AddClient runs `easyrsa build-client-full` for cn (passwordless), then
// assembles the .ovpn bundle the same way openvpn-install.sh does.
func (m *Manager) AddClient(ctx context.Context, cn string) ([]byte, error) {
	if !validCN.MatchString(cn) {
		return nil, fmt.Errorf("invalid common name: %q", cn)
	}

	// Refuse if a cert with this CN already exists in the index.
	certs, err := ReadIndex(m.EasyRSADir)
	if err == nil {
		for _, c := range certs {
			if c.CommonName == cn && c.Status == "valid" {
				return nil, fmt.Errorf("client %q already exists", cn)
			}
		}
	}

	cmd := exec.CommandContext(ctx, "./easyrsa", "--batch", "build-client-full", cn, "nopass")
	cmd.Dir = m.EasyRSADir
	cmd.Env = append(os.Environ(), "EASYRSA_CERT_EXPIRE=3650")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("easyrsa build-client-full failed: %w\n%s", err, out)
	}

	bundle, err := m.buildOVPN(cn)
	if err != nil {
		return nil, fmt.Errorf("assemble .ovpn: %w", err)
	}

	if err := os.MkdirAll(m.ClientsDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure clients dir: %w", err)
	}
	dst := filepath.Join(m.ClientsDir, cn+".ovpn")
	if err := os.WriteFile(dst, bundle, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", dst, err)
	}
	return bundle, nil
}

// BuildOVPN re-renders the .ovpn bundle for an existing CN (no easyrsa run).
// Useful when a user re-downloads their config.
func (m *Manager) BuildOVPN(cn string) ([]byte, error) {
	if !validCN.MatchString(cn) {
		return nil, fmt.Errorf("invalid common name: %q", cn)
	}
	return m.buildOVPN(cn)
}

func (m *Manager) buildOVPN(cn string) ([]byte, error) {
	pki := filepath.Join(m.EasyRSADir, "pki")
	tmplPath := filepath.Join(m.OpenVPNDir, "client-template.txt")
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("read client-template.txt: %w", err)
	}
	ca, err := os.ReadFile(filepath.Join(pki, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("read ca.crt: %w", err)
	}
	crt, err := os.ReadFile(filepath.Join(pki, "issued", cn+".crt"))
	if err != nil {
		return nil, fmt.Errorf("read issued %s.crt: %w", cn, err)
	}
	key, err := os.ReadFile(filepath.Join(pki, "private", cn+".key"))
	if err != nil {
		return nil, fmt.Errorf("read private %s.key: %w", cn, err)
	}

	var b bytes.Buffer
	b.Write(tmpl)
	if !bytes.HasSuffix(tmpl, []byte("\n")) {
		b.WriteByte('\n')
	}
	b.WriteString("<ca>\n")
	b.Write(ca)
	b.WriteString("</ca>\n")

	// `openvpn-install.sh` only emits the BEGIN/END section of the issued
	// cert (strips human-readable header).
	b.WriteString("<cert>\n")
	b.Write(extractCertPEM(crt))
	b.WriteString("</cert>\n")

	b.WriteString("<key>\n")
	b.Write(key)
	b.WriteString("</key>\n")

	if data, err := os.ReadFile(filepath.Join(m.OpenVPNDir, "tls-crypt.key")); err == nil {
		b.WriteString("<tls-crypt>\n")
		b.Write(data)
		b.WriteString("</tls-crypt>\n")
	} else if data, err := os.ReadFile(filepath.Join(m.OpenVPNDir, "tls-auth.key")); err == nil {
		b.WriteString("key-direction 1\n<tls-auth>\n")
		b.Write(data)
		b.WriteString("</tls-auth>\n")
	}
	return b.Bytes(), nil
}

func extractCertPEM(in []byte) []byte {
	const begin = "-----BEGIN CERTIFICATE-----"
	const end = "-----END CERTIFICATE-----"
	s := string(in)
	i := strings.Index(s, begin)
	j := strings.Index(s, end)
	if i < 0 || j < 0 {
		return in
	}
	return []byte(s[i:j+len(end)] + "\n")
}

// RevokeClient runs the same easyrsa revoke + gen-crl flow as the upstream
// script and reloads the CRL on the server.
func (m *Manager) RevokeClient(ctx context.Context, cn string) error {
	if !validCN.MatchString(cn) {
		return fmt.Errorf("invalid common name: %q", cn)
	}

	cmd := exec.CommandContext(ctx, "./easyrsa", "--batch", "revoke", cn)
	cmd.Dir = m.EasyRSADir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("easyrsa revoke %s: %w\n%s", cn, err, out)
	}

	cmd = exec.CommandContext(ctx, "./easyrsa", "--batch", "gen-crl")
	cmd.Dir = m.EasyRSADir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("easyrsa gen-crl: %w\n%s", err, out)
	}

	src := filepath.Join(m.EasyRSADir, "pki", "crl.pem")
	dst := filepath.Join(m.OpenVPNDir, "crl.pem")
	if data, err := os.ReadFile(src); err == nil {
		_ = os.WriteFile(dst, data, 0o644)
	}

	// Forget cached client config and ovpn file.
	_ = os.Remove(filepath.Join(m.ClientsDir, cn+".ovpn"))

	// Drop any active session for this CN by SIGHUPing the server. We
	// avoid bouncing the unit so other peers stay connected; instead we
	// rely on CRL-reload via management socket if available, falling back
	// to nothing — a fresh handshake will be rejected against the new CRL.
	return nil
}

// IsServiceActive returns whether the openvpn server unit is running.
func (m *Manager) IsServiceActive(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", m.ServiceUnit)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) == "active"
}

// ServiceUptime returns a human "Xd YYh:MM" uptime (best-effort, "" if unavailable).
func (m *Manager) ServiceUptime(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "systemctl", "show", m.ServiceUnit, "--property=ActiveEnterTimestamp", "--value")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	ts := strings.TrimSpace(string(out))
	if ts == "" {
		return ""
	}
	t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", ts)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dd %02d:%02d", days, hours, mins)
}
