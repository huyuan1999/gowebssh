# gowebssh

部分核心代码来源于 https://github.com/dejavuzhou/felix

```go
// 在 main.go 中配置连接信息, 然后在浏览器输入 http://127.0.0.1:8088/ 开始使用 websocket
sshObj := sshChannel.SSHClientConfig{
    // 支持 passwd 或者 publickey 和 key
    // passwd 表示使用密码登录, publickey 和 key 表示使用秘钥登录
    AuthType:  "passwd",   
    User:      "root",
    Address:   "192.168.3.31",
    Port:      22,

    // 当 AuthType 为 passwd 时, 此字段为登录密码 
    // 当 AuthType 为 publickey 或 key 时, 此字段为秘钥解密密码
    Password:  "",

    // 秘钥字符串, 类型为 []byte
    Publickey: []byte(`-----BEGIN DSA PRIVATE KEY----- .......`),

    // 连接超时时间
    Timeout:   time.Second * 30,
    
    // 打开的终端大小, 行和列
    PtyWidth:  ptyWidth,
    PtyHeight: ptyHeight,
}
```

demo 和 https://github.com/huyuan1999/django-webssh 的 demo 基本相同