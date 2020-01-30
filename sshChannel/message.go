package sshChannel

import "time"

type SSHClientConfig struct {
	AuthType  string // passwd or publickey
	User      string
	Password  string
	Publickey []byte
	Timeout   time.Duration
	Address   string
	Port      int
	PtyWidth  int
	PtyHeight int
}

type msgProtocol struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}
