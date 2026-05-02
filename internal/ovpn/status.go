package ovpn

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Session is one connected client as reported in OpenVPN's status.log.
type Session struct {
	CommonName    string    `json:"user"`
	RealIP        string    `json:"real_ip"`
	VirtualIP     string    `json:"virtual_ip"`
	BytesIn       int64     `json:"bytes_in"`
	BytesOut      int64     `json:"bytes_out"`
	ConnectedAt   time.Time `json:"connected_at"`
	Cipher        string    `json:"cipher,omitempty"`
	PeerID        string    `json:"peer_id,omitempty"`
}

// ReadSessions parses the OpenVPN status file (default v2 layout used by
// `openvpn-install.sh` — comma separated CLIENT_LIST rows).
func ReadSessions(statusPath string) ([]Session, error) {
	f, err := os.Open(statusPath)
	if err != nil {
		return nil, fmt.Errorf("open status.log: %w", err)
	}
	defer f.Close()

	var sessions []Session
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "CLIENT_LIST,") {
			continue
		}
		// CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username,Client ID,Peer ID,Data Channel Cipher
		parts := strings.Split(line, ",")
		if len(parts) < 9 {
			continue
		}
		s := Session{
			CommonName: parts[1],
			RealIP:     parts[2],
			VirtualIP:  parts[3],
		}
		s.BytesIn, _ = strconv.ParseInt(parts[5], 10, 64)
		s.BytesOut, _ = strconv.ParseInt(parts[6], 10, 64)
		if ts, err := strconv.ParseInt(parts[8], 10, 64); err == nil {
			s.ConnectedAt = time.Unix(ts, 0)
		}
		if len(parts) > 11 {
			s.PeerID = parts[11]
		}
		if len(parts) > 12 {
			s.Cipher = parts[12]
		}
		sessions = append(sessions, s)
	}
	return sessions, sc.Err()
}
