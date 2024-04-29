# piping-tunnel
![CI](https://github.com/nwtgck/go-piping-tunnel/workflows/CI/badge.svg)  

Tunneling over HTTP with [Piping Server](https://github.com/nwtgck/piping-server) 


## Install for Windows
[Download](https://github.com/nwtgck/go-piping-tunnel/releases/download/v0.10.1/piping-tunnel-0.10.1-windows-amd64.zip)

## Install for macOS
```bash
brew install nwtgck/piping-tunnel/piping-tunnel
```

## Install for Ubuntu
```bash
wget https://github.com/nwtgck/go-piping-tunnel/releases/download/v0.10.1/piping-tunnel-0.10.1-linux-amd64.deb
sudo dpkg -i piping-tunnel-0.10.1-linux-amd64.deb 
```

Get more executables in the [releases](https://github.com/nwtgck/go-piping-tunnel/releases).

## Help

You can use `$PIPING_SERVER` env to set default Piping Server.

```txt
Tunneling from anywhere with Piping Server

Usage:
  piping-tunnel [flags]
  piping-tunnel [command]

Examples:

Normal:
  piping-tunnel server -p 22 aaa bbb
  piping-tunnel client -p 1022 aaa bbb

Short:
  piping-tunnel server -p 22 mypath
  piping-tunnel client -p 1022 mypath

Multiplexing:
  piping-tunnel server -p 22 --yamux aaa bbb
  piping-tunnel client -p 1022 --yamux aaa bbb

SOCKS proxy like VPN:
  piping-tunnel socks --yamux aaa bbb
  piping-tunnel client -p 1080 --yamux aaa bbb

Environment variable:
  $PIPING_SERVER for default Piping Server


Available Commands:
  client      Run client-host
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  server      Run server-host
  socks       Run SOCKS server

Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
  -h, --help                      help for piping-tunnel
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 4096)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 4096)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
      --verbose int               Verbose logging level
  -v, --version                   show version

Use "piping-tunnel [command] --help" for more information about a command.
```

The following help is for server-host.
```
Run server-host

Usage:
  piping-tunnel server [flags]

Flags:
      --cipher-type string   Cipher type: aes-ctr, openssl-aes-128-ctr, openssl-aes-256-ctr, openpgp  (default "aes-ctr")
      --cs-buf-size uint     Buffer size of client-to-server in bytes (default 4096)
  -h, --help                 help for server
      --host string          Target host (default "localhost")
      --pass string          Passphrase for encryption
      --pbkdf2 string        e.g. {"iter":100000,"hash":"sha256"}
      --pmux                 Multiplex connection by pmux (experimental)
      --pmux-config string   pmux config in JSON (experimental) (default "{\"hb\": true}")
  -p, --port int             TCP port of server host
  -c, --symmetric            Encrypt symmetrically
      --unix-socket string   Unix socket of server host
      --yamux                Multiplex connection by hashicorp/yamux

Global Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 4096)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 4096)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
      --verbose int               Verbose logging level
```

The following help is for client-host.
```
Run client-host

Usage:
  piping-tunnel client [flags]

Flags:
      --cipher-type string   Cipher type: aes-ctr, openssl-aes-128-ctr, openssl-aes-256-ctr, openpgp  (default "aes-ctr")
  -h, --help                 help for client
      --pass string          Passphrase for encryption
      --pbkdf2 string        e.g. {"iter":100000,"hash":"sha256"}
      --pmux                 Multiplex connection by pmux (experimental)
      --pmux-config string   pmux config in JSON (experimental) (default "{\"hb\": true}")
  -p, --port int             TCP port of client host
      --sc-buf-size uint     Buffer size of server-to-client in bytes (default 4096)
  -c, --symmetric            Encrypt symmetrically
      --unix-socket string   Unix socket of client host
      --yamux                Multiplex connection by hashicorp/yamux

Global Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 4096)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 4096)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
      --verbose int               Verbose logging level
```

The following help is for SOCKS proxy.

```
Run SOCKS server

Usage:
  piping-tunnel socks [flags]

Flags:
      --cipher-type string   Cipher type: aes-ctr, openssl-aes-128-ctr, openssl-aes-256-ctr, openpgp  (default "aes-ctr")
  -h, --help                 help for socks
      --pass string          Passphrase for encryption
      --pbkdf2 string        e.g. {"iter":100000,"hash":"sha256"}
      --pmux                 Multiplex connection by pmux (experimental)
      --pmux-config string   pmux config in JSON (experimental) (default "{\"hb\": true}")
  -c, --symmetric            Encrypt symmetrically
      --yamux                Multiplex connection by hashicorp/yamux

Global Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 4096)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 4096)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
      --verbose int               Verbose logging level
```

## References
The idea of tunneling over Piping Server was proposed by [@Cryolite](https://github.com/Cryolite). Thanks!  
- (Japanese) <https://qiita.com/Cryolite/items/ed8fa237dd8eab54ef2f>

## Related work
- [portwarp](https://github.com/essa/portwarp)
