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

## v1.4.0: unix `cp` syntax - globs, braces and multiple paths!

```bash
rbcp log_{1,2}.txt test_* d:/dest
```

### Added
- add support for unix `cp` syntax
	- add mvdan/sh parsing for multiple sources paths, braces, globs, etc.
	- command and process substitution not allowed for now
	- dont replace env vars (robocopy does this on its own)
- add fatal error style

### Changed
- robocopy passthrough arguments now need to be specified with `-[`
- refactor the gigantic main function into setup, parseArgs
- normalize paths to /

### Fixed
- dont start robocopy when nothing to copy 🙃
- check for brace expansion file conflict

---

## v1.3.0: force quit, better summary and logging

This update adds a force quit option that cancels the running robocopy command by pressing <kbd>q</kbd> or <kbd>ctrl+c</kbd>. 

> [!NOTE]
> No data would be lost as robocopy in general does not delete. When it does delete (when using `/MIR`), it waits for a file to be completely copied before deletion, so you will always have at least one complete copy - ROBUST file copy 😄.

### Added
- version injection in goreleaser + better `--version` text
- committed taskfile
- added a force quit mechanism
- added a user-facing error log style

### Changed
- replaced direct `log` calls with global `logger` (on which styles are applied)
- improved struct logging of config and (new) stats by changing `%v` to `%+v`
- changed `exec.Command` to `exec.CommandContext` for cancellation in force quit events

---

## v1.2.2: bugfix, add future config file

### Added
- demo gif to readme, made using vhs (asciicast/PowerSession errors out because of some unicode error, ideally would want that)
- add the `changelog.md` file
- add a toml config file for allowing basic options (in development)
	- use nerd font arrow (which needs any NF to be installed) instead of --> (which depends on font ligatures to be present and enabled). surprisingly there is no good option for a unicode big long arrow. 
	- show progress bar or not 
	- theming options - pretty customizable

### Fixed
- fix `%w` not allowed in (charmbracelet)`log.Errorf`

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
