# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added
### Changed
### Removed
### Fixed

---

## v1.2.1: improvements

### Added
- allow `/log` arg in robocopy
- a basic readme

### Changed
- better user provided arg checking
- minor display improvements
- improve unrecognized exit code handling
- don't force quit
- send msg on summary detected
- always send ProgressMsg at 100%

---

## v1.2.0: add progress per-file

This minor release adds streaming output through pipes, so that we can improve the display of progress in the TUI.

### Added
- had to add a custom scanner that also splits on `\r`.
- added a ratelimiter to reduce progress msg frequency. Currently set to 25ms but could be decreased as well.
- added profiling
- better envvar handling (using os.Getenv now instead of LookupEnv)

### Changed
- change to reading from a pipe
- moved updatemsg processing to update itself instead of different function, added common updatepercent
- completely change output parsing mechanism, change to piping output to make live progress possible

### Removed
- old output parser that waits for robocopy to exit

### Fixed
- also fixed a pesky bug (tui.go#L94) - I was converting to `int` after doing the subtraction and then getting `0`, had to do the whole multiplication as a `float` and then convert to `int`.
- replaced wrong string(<int>) with strconv.Itoa(). Why can't string() be nice?

---

## v1.1.2: minor improvements

### Added
- q/ctrl+c now exits the TUI

### Changed
- removed `/NP` flag from args passed to `robocopy`
- change `fmt.Sprintf` to simpler + faster string concatenation
- return error on reading robocopy output

---

## v1.1.1: fix bug in exitcode evaluation

### Fixed
- fix bug in exitcode evaluation

---

## v1.1.0: huge improvements

### Added
- added proper cli args
- added lipgloss styling to output, highlight important stuff

### Changed
- better summary display
- better exit code handling
- improved exit code "reasoning"

### Fixed
- improved goroutine launching to have minimal waiting time after robocopy is actually done (~0.01 s)
- fix a hanging bug
- formatting and minor refactoring
