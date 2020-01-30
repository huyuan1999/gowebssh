package sshChannel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// write data to WebSocket
// the data comes from ssh server.
type wsBufferWriter struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

// connect to ssh server using ssh session.
type SshConn struct {
	// calling Write() to write data into ssh server
	StdinPipe io.WriteCloser
	// Write() be called to receive data from ssh server
	ComboOutput *wsBufferWriter
	Session     *ssh.Session
}

const (
	sshCmd       = "cmd"
	sshResizePty = "resize"
)

// implement Write interface to write bytes from ssh server into bytes.Buffer.
func (w *wsBufferWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

func baseHostKeyCallback(addr string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if addr == remote.String() {
			return nil
		} else {
			log.Printf("srcconn: %s, dstconn: %s", addr, hostname)
			return fmt.Errorf("Remote host authentication error")
		}
	}
}

func (sshArgs *SSHClientConfig) NewSshClient() (*ssh.Client, error) {
	var addr string
	addr = fmt.Sprintf("%s:%d", sshArgs.Address, sshArgs.Port)
	config := &ssh.ClientConfig{
		User:    sshArgs.User,
		Timeout: sshArgs.Timeout,
	}

	passwd := sshArgs.Password
	if sshArgs.AuthType == "publickey" || sshArgs.AuthType == "key" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(sshArgs.Publickey, []byte(sshArgs.Password))
		if err != nil {
			log.Printf("unable to parse private key: %v", err)
			return nil, fmt.Errorf(err.Error())
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		config.HostKeyCallback = baseHostKeyCallback(addr)
	} else {
		config.Auth = []ssh.AuthMethod{ssh.Password(passwd)}
		config.HostKeyCallback = baseHostKeyCallback(addr)
	}

	client, err := ssh.Dial("tcp", addr, config)
	return client, err
}

func (sshArgs *SSHClientConfig) NewSshConn(sshClient *ssh.Client) (*SshConn, error) {
	sshSession, err := sshClient.NewSession()
	if err != nil {
		return nil, err
	}

	// we set stdin, then we can write data to ssh server via this stdin.
	// but, as for reading data from ssh server, we can set Session.Stdout and Session.Stderr
	// to receive data from ssh server, and write back to somewhere.
	stdinP, err := sshSession.StdinPipe()
	if err != nil {
		return nil, err
	}

	comboWriter := new(wsBufferWriter)
	//ssh.stdout and stderr will write output into comboWriter
	sshSession.Stdout = comboWriter
	sshSession.Stderr = comboWriter

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echo
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err := sshSession.RequestPty("xterm", sshArgs.PtyHeight, sshArgs.PtyWidth, modes); err != nil {
		return nil, err
	}
	// Start remote shell
	if err := sshSession.Shell(); err != nil {
		return nil, err
	}
	return &SshConn{StdinPipe: stdinP, ComboOutput: comboWriter, Session: sshSession}, nil
}

func (s *SshConn) Close() {
	if s.Session != nil {
		s.Session.Close()
	}
}

//flushComboOutput flush ssh.session combine output into websocket response
func flushComboOutput(w *wsBufferWriter, wsConn *websocket.Conn) error {
	if w.buffer.Len() != 0 {
		buffer := w.buffer.Bytes()

		data := &msgProtocol{
			Type: "cmd",
			Data: string(buffer),
		}

		sendCmdResult, _ := json.Marshal(data)
		err := wsConn.WriteMessage(websocket.TextMessage, sendCmdResult)
		if err != nil {
			log.Println(err)
			return err
		}
		w.buffer.Reset()
	}
	return nil
}

//ReceiveWsMsg  receive websocket msg do some handling then write into ssh.session.stdin
func (ssConn *SshConn) ReceiveWsMsg(wsConn *websocket.Conn, logBuff *bytes.Buffer, exitCh chan bool) {
	//tells other go routine quit
	defer setQuit(exitCh)
	for {
		select {
		case <-exitCh:
			return
		default:
			//read websocket msg
			_, wsData, err := wsConn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}

			msgObj := &msgProtocol{}
			err = json.Unmarshal(wsData, msgObj)
			if err != nil {
				log.Println(err)
				return
			}

			switch msgObj.Type {
			case sshResizePty:
				if msgObj.Cols > 0 && msgObj.Rows > 0 {
					if err := ssConn.Session.WindowChange(msgObj.Rows, msgObj.Cols); err != nil {
						log.Println(err)
					}
				}
			case sshCmd:
				cmdByte := []byte(msgObj.Data)
				if _, err := ssConn.StdinPipe.Write(cmdByte); err != nil {
					log.Println(err)
					setQuit(exitCh)
				}
				//write input cmd to log buffer
				if _, err := logBuff.Write(cmdByte); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (ssConn *SshConn) SendComboOutput(wsConn *websocket.Conn, exitCh chan bool) {
	//tells other go routine quit
	defer setQuit(exitCh)

	//every 120ms write combine output bytes into websocket response
	tick := time.NewTicker(time.Millisecond * time.Duration(12))
	// send ping message
	var interval time.Duration
	pingTick := time.NewTimer(interval)
	//for range time.Tick(120 * time.Millisecond){}
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			//write combine output bytes into websocket response
			if err := flushComboOutput(ssConn.ComboOutput, wsConn); err != nil {
				log.Println(err)
				return
			}
		case <-pingTick.C:
			if err := wsConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println(err)
				return
			}

		case <-exitCh:
			return
		}
	}
}

func (ssConn *SshConn) SessionWait(quitChan chan bool) {
	if err := ssConn.Session.Wait(); err != nil {
		log.Println(err)
		setQuit(quitChan)
	}
}

func setQuit(ch chan bool) {
	ch <- true
}
