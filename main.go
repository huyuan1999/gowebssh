package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	"gossh/sshChannel"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"
)

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024 * 10,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	DEFAULT_PTY_WIDTH  = 120
	DEFAULT_PTY_HEIGHT = 40
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func main() {
	static := http.FileServer(http.Dir("static"))
	stripPrefix := http.StripPrefix("/static/", static)
	http.Handle("/static/", stripPrefix)

	http.HandleFunc("/", index)
	http.HandleFunc("/api/v1/ssh/", openWebssh)
	log.Println("start")
	err := http.ListenAndServe(":8088", nil)
	if err != nil {
		log.Panic(err)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Println(err)
	}
	t.Execute(w, nil)
}

func openWebssh(w http.ResponseWriter, r *http.Request) {
	var ptyWidth int
	var ptyHeight int

	if err := r.ParseForm(); err != nil {
		log.Println(err)
		ptyWidth = DEFAULT_PTY_WIDTH
		ptyHeight = DEFAULT_PTY_HEIGHT
	} else {
		width, err1 := strconv.ParseInt(r.FormValue("ptyWidth"), 10, 64)
		height, err2 := strconv.ParseInt(r.FormValue("ptyHeight"), 10, 64)
		if err1 == nil && err2 == nil {
			ptyWidth = int(width)
			ptyHeight = int(height)
		} else {
			ptyWidth = DEFAULT_PTY_WIDTH
			ptyHeight = DEFAULT_PTY_HEIGHT
		}
	}

	// When AuthType is publickey, password is the decryption password of publickey
	sshObj := sshChannel.SSHClientConfig{
		AuthType: "passwd", // passwd or publickey and key
		User:     "root",
		Password: "123.com",
		// Publickey: []byte(TestKey2),
		Timeout:   time.Second * 30,
		Address:   "192.168.3.31",
		Port:      22,
		PtyWidth:  ptyWidth,
		PtyHeight: ptyHeight,
	}

	client, err := sshObj.NewSshClient()
	if err != nil {
		log.Println(err)
		return
	}

	defer client.Close()

	sshConn, err := sshObj.NewSshConn(client)
	if err != nil {
		log.Println(err)
		return
	}

	defer sshConn.Close()

	// after configure, the WebSocket is ok.
	wsConn, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer wsConn.Close()

	quitChan := make(chan bool, 3)

	var logBuff = new(bytes.Buffer)

	// most messages are ssh output, not webSocket input
	go sshConn.ReceiveWsMsg(wsConn, logBuff, quitChan)
	go sshConn.SendComboOutput(wsConn, quitChan)
	go sshConn.SessionWait(quitChan)

	<-quitChan
}
