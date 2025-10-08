# rbcp - Robust Copy with Clean Output

![Go Version](https://img.shields.io/badge/Go-1.x-blue)
[![Version](https://img.shields.io/badge/Version-1.2.2-green)](https://github.com/plutonium-239/rbcp/releases)

`rbcp` is a modern wrapper around `robocopy` that combines the efficiency and robustness of the time-tested Windows `robocopy` tool with a clean, user-friendly interface and simplified output.

_Some glyphs are not rendered properly in the GIF, they depend on you having a nerd font installed (for e.g. `-->`)_
![demo](demo/demo.gif)
<sub>_BTW, this is `cmd.exe` running [`clink`](https://github.com/chrisant996/clink), turbocharging everything from the oh-my-posh prompt to the autocompletion and the syntax highlighting._</sub>

## Features

- 🐚 Familiar bash-y `cp` syntax
- 🚀 Modern real-time progress bar
- 📊 Clean, concise output format with important items highlighted
- 🛡️ Preserves robocopy's legendary reliability
- 🎯 Smart defaults for general operation
- 🌟 Brace expansions, glob expansion and many more (powered by [`mvdan/sh`](https://github.com/mvdan/sh)) 
- 📈 Detailed statistics and performance metrics
- 🔄 Mirror mode support
- 🏃 Dry-run capability

## Installation

### Download a binary from the [latest release](https://github.com/plutonium-239/rbcp/releases/latest).

**OR**

Install using go:

```cmd
go install github.com/plutonium-239/rbcp@latest
```

## Usage

Basic syntax:
```cmd
rbcp SOURCE DESTINATION [OPTIONS]
```

### Syntax - just use linux/`cp` syntax

#### copying file(s) in the current working directory:
```cmd
rbcp C:\source\readme.md D:\destination
```

You can also omit the path if it is the CWD:
```cmd
rbcp readme.md D:\destination
```

#### Copy multiple files:
```cmd
rbcp readme.md snippets.md d:/destination/
```

**Glob expansion**
```cmd
rbcp C:/source/log_*.txt d:/destination/
```

**Brace expansion**
The bash `./a_{1,2}` brace expansion syntax is also supported:
```bash
rbcp C:/source/{readme.md,snippets.md} D:\destination
rbcp log_{1,2}.txt d:/destination/
```
> ![DANGER]
> `{`/`}` and `,` are valid characters in file paths in windows, so there might exist files with the exact path that you input, i.e. `C:/source/test{1,2}` itself is a file name (albeit **very rarely**, mostly only GUIDs and temp files). In these cases, `rbcp` will try and detect if such a file exists and ask for confirmation if you still want to continue. This is a limitation that can be solved by using a different syntax and can be discussed [here](https://github.com/plutonium-239/rbcp/discussions/1).

### Mirror directories:
```cmd
rbcp C:\source D:\destination -m
rbcp C:\source D:\destination --mir
```

### Dry run (list only):
```cmd
rbcp C:\source D:\destination -l
rbcp C:\source D:\destination --list
```

### Command Line Options

- `-m`, `--mir`: Mirror mode (equivalent to robocopy's `/MIR`)
- `-l`, `--list`: List-only mode (dry run)
- `--insane`: Disable sane defaults (currently sets \#retries to 2 and timeout between them to 1 sec)
- `-p`, `--preserve-exitcode`: Preserve robocopy's original exit code. By default, exit with code 0 on success and passthrough on copy failures.
- Additional robocopy arguments can be passed directly to `--passthrough`/`-[`.

## Features

### Smart Defaults

- Optimized retry settings (`/R:2 /W:1`)
- Clean output formatting
- Automatic terminal width detection

### Real-time Progress

- Live progress bar showing:
  - Current file being copied
  - Overall progress
  - Transfer speed
  - Remaining files/bytes

### Summary Statistics

Detailed completion summary including:
- Files copied/skipped
- Directories processed
- Total data transferred
- Transfer speed
- Duration
- Status of any failures or mismatches

## Exit Codes

The tool maintains compatibility with robocopy's exit codes while providing a more user-friendly interpretation:

- 0: No errors
- 1: One or more files copied successfully
- 2: Extra files or directories detected
- 4: Mismatched files or directories found
- 8: Some files or directories could not be copied
- 16: Serious error - no files copied

Note: By default, non-error exit codes (< 8) are converted to 0 unless `--preserve-exitcode` is used.

## Environment Variables

- `LOGLEVEL`: Set logging verbosity level
- `COLUMNS`: Override terminal width detection

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source and available under the [MIT License](LICENSE).

## Author

Created by [plutonium-239](https://github.com/plutonium-239)

## Acknowledgments

Built on top of the robust `robocopy` tool, with modern UI elements powered by:
- [Charm](https://github.com/charmbracelet) libraries for terminal UI
- [go-arg](https://github.com/alexflint/go-arg) for argument parsing