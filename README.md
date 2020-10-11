# piping-tunnel
[![CircleCI](https://circleci.com/gh/nwtgck/go-piping-tunnel.svg?style=shield)](https://app.circleci.com/pipelines/github/nwtgck/go-piping-tunnel)

Tunnel CLI over [Piping Server](https://github.com/nwtgck/piping-server)

## Help

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
      --dns-server string   DNS server (e.g. 1.1.1.1:53)
  -h, --help                help for piping-tunnel
  -k, --insecure            Allow insecure server connections when using SSL
      --progress            Show progress (default true)
  -s, --server string       Piping Server URL (default "https://ppng.io")
  -v, --version             show version

Use "piping-tunnel [command] --help" for more information about a command.
```

## References
- (Japanese) <https://qiita.com/Cryolite/items/ed8fa237dd8eab54ef2f>

## Related work
- [portwarp](https://github.com/essa/portwarp)
