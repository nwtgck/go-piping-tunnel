# piping-tunnel
![CI](https://github.com/nwtgck/go-piping-tunnel/workflows/CI/badge.svg) [![CircleCI](https://circleci.com/gh/nwtgck/go-piping-tunnel.svg?style=shield)](https://app.circleci.com/pipelines/github/nwtgck/go-piping-tunnel)

Tunneling over HTTP with [Piping Server](https://github.com/nwtgck/piping-server) 

## Help

You can use `$PIPING_SERVER` env to set default Piping Server.

```txt
Tunnel over Piping Server

Usage:
  piping-tunnel [flags]
  piping-tunnel [command]

Available Commands:
  client      Run client-host
  help        Help about any command
  server      Run server-host

Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
  -h, --help                      help for piping-tunnel
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 16)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 16)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
  -v, --version                   show version
```

The following help is for server-host.
```
Run server-host

Usage:
  piping-tunnel server [flags]

Flags:
      --c-to-s-buf-size uint   Buffer size of client-to-server in bytes (default 16)
  -h, --help                   help for server
  -p, --port int               TCP port of server host
      --yamux                  Multiplex connection by hashicorp/yamux

Global Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 16)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 16)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
```

The following help is for client-host.
```
Run client-host

Usage:
  piping-tunnel client [flags]

Flags:
  -h, --help                   help for client
  -p, --port int               TCP port of client host
      --s-to-c-buf-size uint   Buffer size of server-to-client in bytes (default 16)
      --yamux                  Multiplex connection by hashicorp/yamux

Global Flags:
      --dns-server string         DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray        HTTP header
      --http-read-buf-size int    HTTP read-buffer size in bytes (default 16)
      --http-write-buf-size int   HTTP write-buffer size in bytes (default 16)
  -k, --insecure                  Allow insecure server connections when using SSL
      --progress                  Show progress (default true)
  -s, --server string             Piping Server URL (default "https://ppng.io")
```

## References
The idea of tunneling over Piping Server was proposed by [@Cryolite](https://github.com/Cryolite). Thanks!  
- (Japanese) <https://qiita.com/Cryolite/items/ed8fa237dd8eab54ef2f>

## Related work
- [portwarp](https://github.com/essa/portwarp)
