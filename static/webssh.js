function termSize() {
    // rows:cols / height:width
    const init_width = 9;
    const init_height = 18;
    // const init_height = 17;

    let windows_width = window.innerWidth;
    let windows_height = window.innerHeight;

    return {
        cols: Math.floor(windows_width / init_width),
        rows: Math.floor(windows_height / init_height),
    }
}


function openSSH(id) {
    let initPtySize = termSize();
    let cols = initPtySize.cols;
    let rows = initPtySize.rows;

    let term;
    term = new Terminal({
        cols: cols,
        rows: rows,
        useStyle: true,
        cursorBlink: true,
    });

    let joinArgs = 'id=' + id + '&ptyWidth=' + cols + '&ptyHeight=' + rows;

    let protocol = (location.protocol === 'https:') ? 'wss://' : 'ws://',
        socketURL = protocol + location.hostname + ((location.port) ? (':' + location.port) : '') +
            '/api/v1/ssh/?' + joinArgs;

    let sock;
    sock = new WebSocket(socketURL);

    // open websocket conn and open webssh term
    sock.addEventListener('open', function () {
        term.open(document.getElementById('terminal'));
    });

    // read data and write in term
    sock.addEventListener('message', function (message) {
        // let myMessage = message;
        let data = JSON.parse(message.data).data;
        term.write(data)
    });

    let msgProtocol = {'type': null, 'data': null, 'cols': null, 'rows': null};

    // send data to server
    term.on('data', function (data) {
        msgProtocol['type'] = "cmd";
        msgProtocol['data'] = data;
        let send_data = JSON.stringify(msgProtocol);
        sock.send(send_data)
    });

    // resize ssh and web term pty size
    window.onresize = function () {
        let currentPtySize = termSize();
        let cols = currentPtySize.cols;
        let rows = currentPtySize.rows;

        msgProtocol['type'] = "resize";
        msgProtocol['cols'] = cols;
        msgProtocol['rows'] = rows;

        let send_data = JSON.stringify(msgProtocol);
        sock.send(send_data);
        term.resize(cols, rows);
    };
}
