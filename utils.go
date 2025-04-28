package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/charmbracelet/log"
	// "github.com/charmbracelet/log"
)

// FileStats represents statistics for a category of files
type FileStats struct {
	Dirs  int
	Files int
	Bytes int64
}

// RobocopyStats represents all statistics from a robocopy operation
type RobocopyStats struct {
	// Categories of statistics
	Total    FileStats
	Copied   FileStats
	Skipped  FileStats
	Mismatch FileStats
	Failed   FileStats
	Extras   FileStats

	// Speed information
	BytesPerSec     int64
	MegaBytesPerMin float64

	// Duration
	Duration time.Duration

	// Exit code
	ExitCode int
}

// improved regex patterns for file detection - to be used in main.go
var (
	// File copying patterns with more specific matches for robocopy output
	reFileCopying = regexp.MustCompile(`^\s*(?:New File|File)\s+(\d+)\s+(.+)`)
	// reFileCopying2 = regexp.MustCompile(`^\s*(\d+)%\s+(.+)`)
	reFileProgress = regexp.MustCompile(`(\d+\.\d+|\d+)\%`)

	// Summary parsing patterns
	reDirs         = regexp.MustCompile(`^\s*Dirs\s*:\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)`)
	reFiles        = regexp.MustCompile(`^\s*Files\s*:\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)`)
	reBytes        = regexp.MustCompile(`^\s*Bytes\s*:\s*([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)`)
	reSpeedBytes   = regexp.MustCompile(`^\s*Speed\s*:\s*(\d+)\s*Bytes\/sec`)
	reSpeedMB      = regexp.MustCompile(`^\s*Speed\s*:\s*([0-9.]+)\s*MegaBytes\/min`)
	reSummaryStart = regexp.MustCompile(`^\s*Total\s+Copied\s+Skipped\s+Mismatch\s+FAILED\s+Extras`)

)

// var progressMsgLimiter = rate.NewLimiter(1000 / 50, 3) // 1000 / N = 1 event per N ms

var progressMsgLimiter = rate.Sometimes{Interval: time.Millisecond*25} // 1 event per N ms
// var progressMsgLimiter = rate.Sometimes{Every: 20} // 1 event per N ms

func parseStreaming(stdout io.Reader, stats *RobocopyStats) error {
	scanner := bufio.NewScanner(stdout)
	split := func (data []byte, atEOF bool) (advance int, token []byte, err error) {
		// similar to bufio.ScanLines but also splits on \r
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		} else if i := bytes.Index(data, []byte("\r\n")); i >= 0 {
			return i + 2, data[0:i], nil
		} else if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[0:i], nil
		} else if i := bytes.IndexByte(data, '\r'); i >= 0 {
			return i + 1, data[0:i], nil
		}

		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return len(data), data, nil
		}

		// Request more data.
		return 0, nil, nil
	}
	scanner.Split(split)
	inSummary := false

	// Replace the parsing section in copyWithProgress() function with:

	var fileSize int64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Debug line to see what's coming from robocopy (uncomment if needed)
		// log.Printf("DEBUG: %s\n", strconv.Quote(line))

		// Check if we're in the summary section
		if reSummaryStart.MatchString(line) {
			// log.Infof("IN SUMMARY : FOUND %v", line)
			inSummary = true
			continue
		}

		if inSummary {
			// # Parse summary information
			// Dirs
			if matches := reDirs.FindStringSubmatch(line); len(matches) > 6 {
				stats.Total.Dirs, _ = strconv.Atoi(matches[1])
				stats.Copied.Dirs, _ = strconv.Atoi(matches[2])
				stats.Skipped.Dirs, _ = strconv.Atoi(matches[3])
				stats.Mismatch.Dirs, _ = strconv.Atoi(matches[4])
				stats.Failed.Dirs, _ = strconv.Atoi(matches[5])
				stats.Extras.Dirs, _ = strconv.Atoi(matches[6])
				continue
			}

			// Files
			if matches := reFiles.FindStringSubmatch(line); len(matches) > 6 {
				stats.Total.Files, _ = strconv.Atoi(matches[1])
				stats.Copied.Files, _ = strconv.Atoi(matches[2])
				stats.Skipped.Files, _ = strconv.Atoi(matches[3])
				stats.Mismatch.Files, _ = strconv.Atoi(matches[4])
				stats.Failed.Files, _ = strconv.Atoi(matches[5])
				stats.Extras.Files, _ = strconv.Atoi(matches[6])
				continue
			}

			// Bytes
			if matches := reBytes.FindStringSubmatch(line); len(matches) > 6 {
				stats.Total.Bytes = parseByteValue(matches[1])
				stats.Copied.Bytes = parseByteValue(matches[2])
				stats.Skipped.Bytes = parseByteValue(matches[3])
				stats.Mismatch.Bytes = parseByteValue(matches[4])
				stats.Failed.Bytes = parseByteValue(matches[5])
				stats.Extras.Bytes = parseByteValue(matches[6])
				continue
			}

			// Speed (Bytes/sec)
			if matches := reSpeedBytes.FindStringSubmatch(line); len(matches) > 1 {
				stats.BytesPerSec, _ = strconv.ParseInt(matches[1], 10, 64)
				continue
			}

			// Speed (MB/min)
			if matches := reSpeedMB.FindStringSubmatch(line); len(matches) > 1 {
				stats.MegaBytesPerMin, _ = strconv.ParseFloat(matches[1], 64)
				continue
			}
		} else {
			// # Try to detect which file is being processed

			// Reset file size for this line
			// fileSize = 0

			// Look for a file size on the line first
			// if sizeMatches := reFileSize.FindStringSubmatch(line); len(sizeMatches) > 1 {
			// 	fileSize = parseByteValue(sizeMatches[1])
			// }

			// Try pattern 1 for file copying
			if matches := reFileCopying.FindStringSubmatch(line); len(matches) > 2 {
				fileSize = parseByteValue(matches[1])
				p.Send(UpdateMsg{matches[2], fileSize, 0})
				// m.UpdateProcessor(matches[2], fileSize)
				continue
			}

			// if !progressMsgLimiter.Allow() {
			// 	log.Infof("trying to match %v: %v", line, reFileProgress.FindStringSubmatch(line))
			// }
			if matches := reFileProgress.FindStringSubmatch(line); len(matches) == 2 {
				// log.Infof("MATCH PROGRESS %v", line)
				progress, err := strconv.ParseFloat(matches[1], 32)
				if err != nil {
					continue
				}
				progressMsgLimiter.Do(func() {
					p.Send(ProgressMsg{fileProg: float32(progress)})
					// p.Printf("macthing %v -> %v", line, matches)
				})
				continue
			}

			// log.Warnf("Could not match line %v", line)

			// // Try pattern 2 for file copying (When /NC is used)
			// if matches := reFileCopying2.FindStringSubmatch(line); len(matches) > 1 {
			// 	p.Send(UpdateMsg{matches[1], true, fileSize})
			// 	continue
			// }
		}
	}
	if err := scanner.Err(); err != nil {
		log.Errorf("error while reading stdout: %v", err)
		return err
	}

	return nil
}


// parseByteValue converts a robocopy byte value string (like "10.5 m") to bytes
func parseByteValue(byteStr string) int64 {
	byteStr = strings.TrimSpace(byteStr)
	if byteStr == "0" || byteStr == "" {
		return 0
	}

	// Extract the number part and the unit (if any)
	parts := strings.Fields(byteStr)
	if len(parts) == 0 {
		return 0
	}

	value, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}

	// Handle units if present
	if len(parts) > 1 {
		unit := strings.ToLower(parts[1])
		switch unit {
		case "k", "kb":
			value *= 1024
		case "m", "mb":
			value *= 1024 * 1024
		case "g", "gb":
			value *= 1024 * 1024 * 1024
		case "t", "tb":
			value *= 1024 * 1024 * 1024 * 1024
		}
	}

	return int64(value)
}

// formatByteValue formats a byte count to a human-readable string
func formatByteValue(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= GB*100:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= MB*100:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= KB*100:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= 100:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// displaySummary outputs the final statistics in a formatted way
func displaySummary(stats RobocopyStats) {
	// #EE6FF8
	fmt.Printf(
		"Copied %s files (%s skipped) over %s directories (%s skipped)\n",
		impStyle.Render(strconv.Itoa(stats.Copied.Files)),
		helpStyle.Render(strconv.Itoa(stats.Skipped.Files)),
		impStyle.Render(strconv.Itoa(stats.Copied.Dirs)),
		helpStyle.Render(strconv.Itoa(stats.Skipped.Dirs)),
	)

	fmt.Printf(
		"resulting in %s data being copied (%s skipped) in %s seconds [%s/s]\n",
		impStyle.Render(formatByteValue(stats.Copied.Bytes)),
		helpStyle.Render(formatByteValue(stats.Skipped.Bytes)),
		impStyle.Render(strconv.FormatFloat(stats.Duration.Seconds(), 'f', 2, 64)),
		helpStyle.Render(formatByteValue(stats.BytesPerSec)),
	)

	if stats.Mismatch.Files > 0 {
		fmt.Printf("Mismatched files: %d\n", stats.Mismatch.Files)
	}
	if stats.Failed.Files > 0 {
		fmt.Printf("Failed files: %d\n", stats.Failed.Files)
	}
	if stats.Extras.Files > 0 {
		fmt.Printf("Extra files: %d\n", stats.Extras.Files)
	}

	// Display exit code and meaning
	ex := "Exit code: " + strconv.Itoa(stats.ExitCode)
	if stats.ExitCode > 8 {
		ex = errorStyle.Render(ex)
	}
	fmt.Println(ex)

	if stats.ExitCode > 0 {
		power := 5
		rem := stats.ExitCode
		for power >= 0 && rem > 0 {
			r := rem >> power
			log.Debugf("exit code iteration power=%d, r=%d, rem=%d", power, r, rem)
			if r > 0 {
				p := PowInt(2, power)
				log.Debugf("printing for p=%d", p)
				explainExitCode(p)
				rem -= p
			}
			power -= 1
		}
	} else {
		explainExitCode(stats.ExitCode)
	}
}

// explainExitCode provides a description of what the robocopy exit code means
func explainExitCode(code int) {
	switch code {
	case 0:
		fmt.Println("No files were copied. No failure was encountered.")
	case 1:
		fmt.Println("One or more files were copied successfully.")
	case 2:
		fmt.Println("Extra files or directories were detected.")
	case 4:
		fmt.Println("Some mismatched files or directories were detected.")
	case 8:
		fmt.Println(errorStyle.Render("Some files or directories could not be copied."))
	case 16:
		fmt.Println(errorStyle.Render("Serious error. Robocopy did not copy any files."))
	default:
		log.Error("Unrecognized status", "exitcode", code)
	}
}

func PowInt(base, exp int) int {
	result := 1
	for {
		if exp&1 == 1 {
			result *= base
		}
		exp >>= 1
		if exp == 0 {
			break
		}
		base *= base
	}

	return result
}
