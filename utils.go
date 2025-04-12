package main

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	// "github.com/charmbracelet/log"
)

// improved regex patterns for file detection - to be used in main.go
var (
	// File copying patterns with more specific matches for robocopy output
	reFileCopying  = regexp.MustCompile(`^\s*(?:New File|File)\s+(\d+)\s+(.+)`)
	// reFileCopying2 = regexp.MustCompile(`^\s*(\d+)%\s+(.+)`)
	
	// Summary parsing patterns
	reDirs         = regexp.MustCompile(`^\s*Dirs\s*:\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)`)
	reFiles        = regexp.MustCompile(`^\s*Files\s*:\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)`)
	reBytes        = regexp.MustCompile(`^\s*Bytes\s*:\s*([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)\s+([0-9.]+\s*[kmgKMG]?)`)
	reSpeedBytes   = regexp.MustCompile(`^\s*Speed\s*:\s*(\d+)\s*Bytes\/sec`)
	reSpeedMB      = regexp.MustCompile(`^\s*Speed\s*:\s*([0-9.]+)\s*MegaBytes\/min`)
	reSummaryStart = regexp.MustCompile(`^\s*Total\s+Copied\s+Skipped\s+Mismatch\s+FAILED\s+Extras`)
)

// parseRobocopyOutput processes the output text from robocopy and extracts statistics
func parseRobocopyOutput(output string, stats *RobocopyStats) error {
	scanner := bufio.NewScanner(strings.NewReader(output))
	inSummary := false

	// Replace the parsing section in copyWithProgress() function with:

	var fileSize int64

	for scanner.Scan() {
		line := scanner.Text()
		
		// Debug line to see what's coming from robocopy (uncomment if needed)
		// log.Printf("DEBUG: %s\n", line)

		// Check if we're in the summary section
		if reSummaryStart.MatchString(line) {
			// log.Infof("IN SUMMARY : FOUND %v", line)
			inSummary = true
			continue
		}

		if inSummary {
			// Parse summary information - no changes to this section
			// Parse Dirs line
			if matches := reDirs.FindStringSubmatch(line); len(matches) > 6 {
				stats.Total.Dirs, _ = strconv.Atoi(matches[1])
				stats.Copied.Dirs, _ = strconv.Atoi(matches[2])
				stats.Skipped.Dirs, _ = strconv.Atoi(matches[3])
				stats.Mismatch.Dirs, _ = strconv.Atoi(matches[4])
				stats.Failed.Dirs, _ = strconv.Atoi(matches[5])
				stats.Extras.Dirs, _ = strconv.Atoi(matches[6])
				continue
			}

			// Parse Files line
			if matches := reFiles.FindStringSubmatch(line); len(matches) > 6 {
				stats.Total.Files, _ = strconv.Atoi(matches[1])
				stats.Copied.Files, _ = strconv.Atoi(matches[2])
				stats.Skipped.Files, _ = strconv.Atoi(matches[3])
				stats.Mismatch.Files, _ = strconv.Atoi(matches[4])
				stats.Failed.Files, _ = strconv.Atoi(matches[5])
				stats.Extras.Files, _ = strconv.Atoi(matches[6])
				continue
			}

			// Parse Bytes line
			if matches := reBytes.FindStringSubmatch(line); len(matches) > 6 {
				stats.Total.Bytes = parseByteValue(matches[1])
				stats.Copied.Bytes = parseByteValue(matches[2])
				stats.Skipped.Bytes = parseByteValue(matches[3])
				stats.Mismatch.Bytes = parseByteValue(matches[4])
				stats.Failed.Bytes = parseByteValue(matches[5])
				stats.Extras.Bytes = parseByteValue(matches[6])
				continue
			}

			// Parse Speed (Bytes/sec)
			if matches := reSpeedBytes.FindStringSubmatch(line); len(matches) > 1 {
				stats.BytesPerSec, _ = strconv.ParseInt(matches[1], 10, 64)
				continue
			}

			// Parse Speed (MB/min)
			if matches := reSpeedMB.FindStringSubmatch(line); len(matches) > 1 {
				stats.MegaBytesPerMin, _ = strconv.ParseFloat(matches[1], 64)
				continue
			}
		} else {
			// Try to detect which file is being processed
			
			// Reset file size for this line
			// fileSize = 0
			
			// Look for a file size on the line first
			// if sizeMatches := reFileSize.FindStringSubmatch(line); len(sizeMatches) > 1 {
			// 	fileSize = parseByteValue(sizeMatches[1])
			// }
			
			// Try pattern 1 for file copying
			if matches := reFileCopying.FindStringSubmatch(line); len(matches) > 2 {
				fileSize = parseByteValue(matches[1])
				p.Send(UpdateMsg{matches[2], fileSize})
				// m.UpdateProcessor(matches[2], fileSize)
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
	fmt.Println("\n========== Summary ==========")
	
	// Display file statistics
	fmt.Printf("Total directories: %d\n", stats.Total.Dirs)
	fmt.Printf("Copied directories: %d\n", stats.Copied.Dirs)
	
	fmt.Printf("Total files: %d\n", stats.Total.Files)
	fmt.Printf("Copied files: %d\n", stats.Copied.Files)
	fmt.Printf("Skipped files: %d\n", stats.Skipped.Files)
	
	if stats.Mismatch.Files > 0 {
		fmt.Printf("Mismatched files: %d\n", stats.Mismatch.Files)
	}
	if stats.Failed.Files > 0 {
		fmt.Printf("Failed files: %d\n", stats.Failed.Files)
	}
	if stats.Extras.Files > 0 {
		fmt.Printf("Extra files: %d\n", stats.Extras.Files)
	}
	
	// Display size information
	fmt.Printf("Total size: %s\n", formatByteValue(stats.Total.Bytes))
	fmt.Printf("Copied size: %s\n", formatByteValue(stats.Copied.Bytes))
	
	// Display duration and speed
	fmt.Printf("Duration: %.2f seconds\n", stats.Duration.Seconds())
	
	if stats.BytesPerSec > 0 {
		mbPerSec := float64(stats.BytesPerSec) / (1024 * 1024)
		fmt.Printf("Speed: %s/sec (%.2f MB/sec)\n", formatByteValue(stats.BytesPerSec), mbPerSec)
	}
	
	// Display exit code and meaning
	fmt.Printf("Exit code: %d\n", stats.ExitCode)
	explainExitCode(stats.ExitCode)
}

// explainExitCode provides a description of what the robocopy exit code means
func explainExitCode(code int) {
	switch {
	case code == 0:
		fmt.Println("No files were copied. No failure was encountered.")
	case code == 1:
		fmt.Println("One or more files were copied successfully.")
	case code == 2:
		fmt.Println("Extra files or directories were detected.")
	case code == 4:
		fmt.Println("Some mismatched files or directories were detected.")
	case code == 8:
		fmt.Println("Some files or directories could not be copied.")
	case code == 16:
		fmt.Println("Serious error. Robocopy did not copy any files.")
	default:
		fmt.Println("Multiple conditions are true (exit code is a combination of the above values).")
	}
}