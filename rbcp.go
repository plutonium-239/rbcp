package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// # Version information
const (
	ProgramName = "rbcp"
	Version     = "1.1.0"
	BuildDate   = "2025-04-17"
	Author      = "plutonium-239"
)

var p *tea.Program

type Args struct {
	Src              string   `arg:"positional, required"`
	Dest             string   `arg:"positional, required"`
	Mir              bool     `arg:"-m" help:"Convenience argument to specify /MIR to robocopy"`
	List             bool     `arg:"-l" help:"Only list files that would be copied. Similar to a 'dry-run' "`
	PreserveExitCode bool     `arg:"-p,--preserve-exitcode" help:"Always return the error code given by robocopy. By default, exit with code 0 on success and passthrough on copy failures."`
	Insane           bool     `help:"Don't set sane defaults (currently sets #retries to 2 and timeout between them to 1 sec."`
	OtherArgs        []string `arg:"positional" help:"All other arguments are passed directly to robocopy."`
	// !!! DISABLE IN PROD
	Profile bool
}

func (Args) Description() string {
	b := ProgramName + " version " + Version + "\n"
	b += "\nrbcp is a compact wrapper around robocopy, aiming to modernize the output while preserving the robustness of this time tested tool."
	b += "\nAll other arguments are passed directly to robocopy."
	return b
}

func (Args) Version() string {
	return ProgramName + " version " + Version
}

func (args Args) buildRobocopyArgs() []string {
	out := []string{args.Src, args.Dest}
	out = append(out, args.OtherArgs...)
	if args.Mir {
		out = append(out, "/MIR")
	}
	return out
}

// func parseArgs(arglist []string) ([]string, error) {
// 	convertedArgs := make([]string, 0)
// 	in, out := "", ""
// 	for _, arg := range arglist {
// 		if strings.HasPrefix(arg, "-") {
// 			option := strings.TrimPrefix(arg, "-")
// 			switch option {
// 			case "-help", "h":
// 				displayHelp()
// 				os.Exit(0)
// 				break
// 			case "-mir", "m":
// 				convertedArgs = append(convertedArgs, "/MIR")
// 				break
// 			case "-list", "-l":
// 				convertedArgs = append(convertedArgs, "/L")
// 				break
// 				// case
// 			}
// 		} else if strings.HasPrefix(arg, "/") {
// 			convertedArgs = append(convertedArgs, arg)
// 		} else if in == "" {
// 			in = arg
// 		} else if out == "" {
// 			out = arg
// 		} else {
// 			log.Fatal("Unrecognized argument: " + arg)
// 			return nil, errors.New("unrecognized args passed")
// 		}
// 	}
// 	return append([]string{in, out}, convertedArgs...), nil
// }

func main() {
	var args Args
	arg.MustParse(&args)

	// TODO: config file for preferences?

	if envLoglvl := os.Getenv("LOGLEVEL"); envLoglvl != "" {
		if lvl, err := log.ParseLevel(envLoglvl); err == nil {
			log.SetLevel(lvl)
		}
	}
	initWidth := 80
	if envColumns := os.Getenv("COLUMNS"); envColumns != "" {
		if i, err := strconv.Atoi(envColumns); err == nil {
			initWidth = i
		}
	}


	if args.Profile {
		os.MkdirAll("prof/", os.ModeDir)
		runtime.SetBlockProfileRate(1)
		t_now := time.Now().Format("2006-01-02 15.04.05")
		f, err := os.Create("prof/" + t_now + ".pprof")
		f2, err2 := os.Create("prof/block_" + t_now + ".pprof")
		if err != nil || err2 != nil {
			log.Fatalf("could not create CPU profile: %v", err)
		}
		defer f.Close()
		defer f2.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("could not start CPU profile: %v", err)
		}
		pprof.Lookup("block").WriteTo(f2, 0)
		defer pprof.StopCPUProfile()
	}

	// TODO: maybe check if user is passing a file as input/dest arg and wants to just copy one file
	// - writing rbcp <path1> <path2> filename doesn't make much sense and doesnt allow cmdline helpers to suggest filenames
	// arrow := pathStyle.Italic(false).Render(" ─── ")
	arrow := pathStyle.Italic(false).Render(" --> ")
	fmt.Println(lipgloss.PlaceHorizontal(initWidth, lipgloss.Center, pathStyle.Render(args.Src) + arrow + pathStyle.Render(args.Dest)))
	rbarglist := args.buildRobocopyArgs()
	log.Infof("Starting compact robocopy with arguments: %v", rbarglist)
	startTime := time.Now()

	if !args.List {
		// Add our output formatting flags
		notAllowed := []string{"/bytes", "/np", "/njh", "/njs", "/ndl", "/nfl", "/ns"}
		slices.DeleteFunc(rbarglist, func(e string) bool {
			return slices.Contains(notAllowed, strings.ToLower(e))
		})
		// let user log if wanted, but we need output to function so tee it
		for _, e := range rbarglist {
			e = strings.ToLower(e)
			if strings.HasPrefix(e, "/log") || strings.HasPrefix(e, "/unilog") {
				rbarglist = append(rbarglist, "/tee")
			}
		}
		rbarglist = append(rbarglist, "/NJH", "/NDL", "/BYTES")
		if !args.Insane {
			rbarglist = append(rbarglist, "/R:2", "/W:1")
		}
	} else {
		fmt.Println()
	}

	totalFiles, totalBytes, err := getTotalCounts(args)
	if err != nil {
		log.Fatalf("Error getting total counts: %v", err)
	}
	log.Infof("Total to copy: %d files, %s\n", totalFiles, formatByteValue(totalBytes))

	m := model{
		progress:   progress.New(progress.WithDefaultGradient(), progress.WithSpringOptions(40, 1)),
		totalFiles: totalFiles,
		totalBytes: totalBytes,
		totalWidth: initWidth,
	}
	// Start Bubble Tea
	p = tea.NewProgram(m)

	// Start the download
	var stats RobocopyStats
	// this apparently makes a 0-memory channel
	ended := make(chan struct{})
	go func() {
		if totalBytes > 0 {
			if _, err := p.Run(); err != nil {
				log.Fatal("error running program:", err)
				os.Exit(1)
			}
		} else {
			log.Info("Nothing to copy, skipping progress bar")
		}
		ended <- struct{}{}
		// log.Warnf("program is over, waiting for summary")
	}()

	// Run robocopy and parse results
	robocopyStart := time.Now()
	stats, err = runRobocopy(rbarglist)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	// log.Debugf("Killed")
	robocopyEnd := time.Now()
	if totalBytes > 0 {
		p.Send(tea.Quit())
		p.Wait()
	}

	<-ended
	// Display summary
	log.Infof("Robocopy took %v", robocopyEnd.Sub(robocopyStart))
	log.Infof("Waited for %v", time.Since(robocopyEnd))
	displaySummary(stats)

	timeTaken := time.Since(startTime)
	log.Infof("Whole program took %v", timeTaken)

	if args.PreserveExitCode || stats.ExitCode >= 8 {
		// Exit with the same code as robocopy
		os.Exit(stats.ExitCode)
	}
}

func runRobocopy(args []string) (RobocopyStats, error) {
	var stats RobocopyStats

	// Start timing
	startTime := time.Now()

	// Run robocopy and capture output
	cmd := exec.Command("robocopy", args...)
	log.Debugf("Starting command %v", cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return stats, fmt.Errorf("failed to get stdout pipe: %v", err)
	}
	// stderr, err := cmd.StderrPipe()
	// if err != nil {
	// 	return stats, fmt.Errorf("failed to get stderr pipe: %v", err)
	// }

	if err := cmd.Start(); err != nil {
		return stats, fmt.Errorf("failed to start robocopy: %v", err)
	}

	ended := make(chan struct{})
	var parsingTime time.Duration 
	go func() {
		parsingStart := time.Now()
		err = parseStreaming(stdout, &stats)
		parsingTime = time.Since(parsingStart)
		ended <- struct{}{}
	}()

	// Calculate duration
	<- ended
	cmd.Wait()
	endTime := time.Now()
	stats.Duration = endTime.Sub(startTime)
	stats.ExitCode = cmd.ProcessState.ExitCode()
	log.Infof("parsing took %v", parsingTime)
	log.Infof("Waited after cmd exit for parsing for %v", time.Since(endTime))

	// Non-fatal error handling (robocopy uses exit codes for normal operations)
	if err != nil && stats.ExitCode > 16 {
		return stats, fmt.Errorf("robocopy failed with exit code %d: %v", stats.ExitCode, err)
	}

	// Parse the output
	// if err := parseRobocopyOutput(string(output), &stats); err != nil {
	// 	return stats, err
	// }

	return stats, nil
}

// getTotalCounts runs robocopy in list-only mode to get total files and bytes
func getTotalCounts(args Args) (int, int64, error) {
	// Clone args and add /L to make it "list only" mode
	rbargs := args.buildRobocopyArgs()
	listArgs := make([]string, len(rbargs))
	copy(listArgs, rbargs)
	if !args.List {
		listArgs = append(listArgs, "/L", "/NFL", "/NDL", "/NP", "/NC", "/BYTES")
	}

	cmd := exec.Command("robocopy", listArgs...)
	output, err := cmd.CombinedOutput()
	if args.List {
		fmt.Print(string(output))
		os.Exit(0)
	}

	var stats RobocopyStats
	if err != nil && cmd.ProcessState.ExitCode() > 16 {
		return 0, 0, fmt.Errorf("robocopy failed with exit code %d: %v", cmd.ProcessState.ExitCode(), err)
	}
	// stderr, err := cmd.StderrPipe()
	// if err != nil {
	// 	return stats, fmt.Errorf("failed to get stderr pipe: %v", err)
	// }

	// if err := cmd.Start(); err != nil {
	// 	return stats, fmt.Errorf("failed to start robocopy: %v", err)
	// }

	err = parseStreaming(bytes.NewReader(output), &stats)

	if err != nil {
		return 0, 0, err
	}

	return stats.Copied.Files, stats.Copied.Bytes, nil
}
