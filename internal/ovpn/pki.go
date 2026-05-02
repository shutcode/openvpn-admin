// Package ovpn wraps the host's openvpn-install.sh installation:
// reads PKI state, parses the live status file, and shells out to easyrsa
// to add/revoke clients. It mirrors the behaviour of the upstream script
// so the WebUI manages the same state the CLI does.
package ovpn

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cert represents one row of easy-rsa's pki/index.txt.
type Cert struct {
	CommonName string    `json:"cn"`
	Serial     string    `json:"serial"`
	Status     string    `json:"status"` // "valid" | "revoked" | "expired"
	NotAfter   time.Time `json:"not_after"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`
}

// IndexPath is the canonical PKI index file.
func IndexPath(easyRSADir string) string {
	return filepath.Join(easyRSADir, "pki", "index.txt")
}

// ReadIndex parses pki/index.txt and returns one Cert per row.
//
// Each row looks like:
//
//	V<TAB>350206112256Z<TAB><TAB>2A02...<TAB>unknown<TAB>/CN=chris
//	R<TAB>...exp..<TAB>...rev..<TAB>SERIAL<TAB>unknown<TAB>/CN=foo
func ReadIndex(easyRSADir string) ([]Cert, error) {
	f, err := os.Open(IndexPath(easyRSADir))
	if err != nil {
		return nil, fmt.Errorf("open index.txt: %w", err)
	}
	defer f.Close()

	var out []Cert
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}
		c := Cert{
			Serial: strings.TrimSpace(fields[3]),
		}
		switch fields[0] {
		case "V":
			c.Status = "valid"
		case "R":
			c.Status = "revoked"
		case "E":
			c.Status = "expired"
		default:
			continue
		}
		c.NotAfter = parsePKITime(fields[1])
		if c.Status == "revoked" {
			c.RevokedAt = parsePKITime(fields[2])
		}
		// CN field looks like "/CN=foo" (or with extra DN attrs).
		dn := fields[5]
		for _, part := range strings.Split(dn, "/") {
			if strings.HasPrefix(part, "CN=") {
				c.CommonName = strings.TrimPrefix(part, "CN=")
				break
			}
		}
		if c.CommonName == "" {
			continue
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// parsePKITime decodes easyrsa's YYMMDDHHMMSSZ time stamps. The format is
// the OpenSSL/ASN.1 UTCTime layout with a 2-digit year.
func parsePKITime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// 350206112256Z → 2035-02-06T11:22:56Z
	t, err := time.Parse("060102150405Z", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ReadStaticIPs parses ipp.txt (and optional ccd/<cn> overrides) and returns
// a CN -> static IP map. ipp.txt rows look like "alice,10.8.0.42".
func ReadStaticIPs(openVPNDir string) map[string]string {
	out := map[string]string{}
	if data, err := os.ReadFile(filepath.Join(openVPNDir, "ipp.txt")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) == 2 {
				out[parts[0]] = parts[1]
			}
		}
	}
	// CCD `ifconfig-push` overrides ipp.txt.
	ccd := filepath.Join(openVPNDir, "ccd")
	if entries, err := os.ReadDir(ccd); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(ccd, e.Name()))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "ifconfig-push ") {
					f := strings.Fields(line)
					if len(f) >= 2 {
						out[e.Name()] = f[1]
					}
				}
			}
		}
	}
	return out
}
