# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)

## [Unreleased]

## [0.6.0] - 2020-12-26
### Changed
* (internal) Improve performance when showing the progress bar
* (internal) Improve performance when using --yamux, reducing unnecessary buffers

## [0.5.0] - 2020-12-26
### Added
* Add "socks" subcommand for SOCKS5 proxy

## [0.4.2] - 2020-12-11
### Changed
* No change (for release)

## [0.4.1] - 2020-12-08
### Changed
* (internal) Specify buffer sizes

### Fixed
* Fix hint to show socat hint when --yamux not specified

## [0.4.0] - 2020-12-06
### Added
* Multiplexing with [hashicorp/yamux](https://github.com/hashicorp/yamux) and add --yamux flag

### Changed
* Use ".../cs" and ".../sc" when the number of paths is one for short
* Rename "$PIPING_SERVER_URL" to "$PIPING_SERVER"

## [0.3.1] - 2020-11-29
### Changed
* Update dependencies

## [0.3.0] - 2020-11-04
### Added
* Add --http-write-buf-size
* Add --http-read-buf-size
* Add --c-to-s-buf-size to client host

## [0.2.2] - 2020-10-29
### Added
* Add --s-to-c-buf-size flag to client

## [0.2.1] - 2020-10-18
### Added
* Add --header flag to specify HTTP header

## [0.2.0] - 2020-10-12
### Changed
* Change server-host as "server" subcommand, not root command
* Allow one rest argument to specify path

### Added
* Create "client" subcommand
* Create --progress flag to show upload/download progress (default: true)

## 0.1.0 - 2020-10-01
### Added
* Initial release

[Unreleased]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.4.2...v0.5.0
[0.4.2]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.1.0...v0.2.0
