package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

// # Version information
const (
	ProgramName    = "compact-robocopy"
	Version        = "1.0.0"
	BuildDate      = "2025-03-10 18:58:31"
	AuthorLogin    = "plutonium-239"
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

var p *tea.Program

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("%s version %s\n", ProgramName, Version)
		fmt.Println("Usage: compact-robocopy [robocopy arguments]")
		fmt.Println("All arguments are passed directly to robocopy.")
		os.Exit(1)
	}

	// Pass all arguments directly to robocopy
	args := os.Args[1:]
	
	fmt.Println("Starting compact robocopy...")
	fmt.Printf("Arguments: %s\n", strings.Join(args, " "))
	fmt.Println()
	if env, found := os.LookupEnv("LOGLEVEL"); found {
		if lvl, err := log.ParseLevel(env); err == nil {
			log.SetLevel(lvl)
		}
	}
	startTime := time.Now()
	
	// Add our output formatting flags
	args = append(args, "/NJH", "/NDL", "/NP", "/BYTES")

	totalFiles, totalBytes, err := getTotalCounts(args)
	if err != nil {
		log.Fatalf("Error getting total counts: %v", err)
	}
	log.Printf("Total to copy: %d files, %s\n", totalFiles, formatByteValue(totalBytes))


	m := model{
		progress: progress.New(progress.WithDefaultGradient(), progress.WithSpringOptions(40, 1)),
		totalFiles: totalFiles,
		totalBytes: totalBytes,
	}
	// Start Bubble Tea
	p = tea.NewProgram(m)

	// Start the download
	var stats RobocopyStats
	go func() {
		// Run robocopy and parse results
		stats, err = runRobocopy(args)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		log.Debugf("Killed")
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("error running program:", err)
		os.Exit(1)
	}
	log.Warnf("program is over, waiting for summary")

	// Display summary
	displaySummary(stats)

	timeTaken := time.Since(startTime)
	log.Infof("Whole program took %v", timeTaken)

	// Exit with the same code as robocopy
	os.Exit(stats.ExitCode)
}

func runRobocopy(args []string) (RobocopyStats, error) {
	var stats RobocopyStats
	
	// Start timing
	startTime := time.Now()

	// Run robocopy and capture output
	cmd := exec.Command("robocopy", args...)
	output, err := cmd.CombinedOutput()
	
	// Calculate duration
	stats.Duration = time.Since(startTime)
	stats.ExitCode = cmd.ProcessState.ExitCode()

	// Non-fatal error handling (robocopy uses exit codes for normal operations)
	if err != nil && stats.ExitCode > 16 {
		return stats, fmt.Errorf("robocopy failed with exit code %d: %v", stats.ExitCode, err)
	}

	// Parse the output
	if err := parseRobocopyOutput(string(output), &stats); err != nil {
		return stats, err
	}

	return stats, nil
}


// getTotalCounts runs robocopy in list-only mode to get total files and bytes
func getTotalCounts(args []string) (int, int64, error) {
	// Clone args and add /L to make it "list only" mode
	listArgs := make([]string, len(args))
	copy(listArgs, args)
	listArgs = append(listArgs, "/L", "/NFL", "/NDL", "/NP", "/NC", "/BYTES")

	cmd := exec.Command("robocopy", listArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil && cmd.ProcessState.ExitCode() > 16 {
		return 0, 0, fmt.Errorf("robocopy failed with exit code %d: %v", cmd.ProcessState.ExitCode(), err)
	}

	var stats RobocopyStats
	err = parseRobocopyOutput(string(output), &stats)
	if err != nil {
		return 0, 0, err
	}

	return stats.Total.Files, stats.Total.Bytes, nil
}