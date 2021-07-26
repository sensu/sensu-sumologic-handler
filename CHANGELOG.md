# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased

### Changed
- Refactor log/metric mode selection using new set of cmdline options.
- Remove metric format option, no need to support anything but prometheus
- Modify sensu events passed as logs into http source  to use 13 digit millisecond accuracy timstamp meeting requirements for automatic timestamp field detection.

## [0.1.1] - 2021-07-14

### Fixed
- Fix crash for golang Event object with nil Metrics

## [0.1.0] - 2021-06-25

### Added
- Initial release
