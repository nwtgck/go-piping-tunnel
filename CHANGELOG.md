# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)

## [Unreleased]

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

[Unreleased]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/nwtgck/go-piping-tunnel/compare/v0.1.0...v0.2.0
