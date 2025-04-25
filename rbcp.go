package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
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
	// Pass all arguments directly to robocopy
	// args := os.Args[1:]
	var args Args
	arg.MustParse(&args)
	// isHelp := slices.ContainsFunc(args, func(e string) bool {
	// 	match, err := regexp.MatchString(`--help|-h`, e)
	// 	return err == nil && match
	// })
	// if len(args) < 3 {
	// 	displayHelp()
	// 	os.Exit(1)
	// }
	// args, err := parseArgs(args)
	// if err != nil {
	// 	os.Exit(1)
	// }

	if env, found := os.LookupEnv("LOGLEVEL"); found {
		if lvl, err := log.ParseLevel(env); err == nil {
			log.SetLevel(lvl)
		}
	}

	// TODO: parse args and figure out dirs vs real args
	// - [ ] ignore args that can't be propagated (/NJH, /NDL, /NP, /BYTES)
	// - [x] add sane defaults set (retries, timeouts etc.)
	// - [x] also add custom args such as
	// - [x] --preserve-exitcode, -p
	// - [x] --mir, -m as convenience
	// - [x] --list, -l
	// - [x] --insane to ignore sane defaults
	fmt.Println(pathStyle.Render(args.Src, " ──── ", args.Dest))
	rbarglist := args.buildRobocopyArgs()
	log.Infof("Starting compact robocopy with arguments: %v", rbarglist)
	startTime := time.Now()

	if !args.List {
		// Add our output formatting flags
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

	// This is so simple but looks so bad
	envColumns, ok := os.LookupEnv("COLUMNS")
	var initWidth int
	if i, err := strconv.Atoi(envColumns); ok && err == nil {
		initWidth = i
	}

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
	ended := make(chan bool)
	go func() {
		if totalBytes > 0 {
			if _, err := p.Run(); err != nil {
				log.Fatal("error running program:", err)
				os.Exit(1)
			}
		} else {
			log.Info("Nothing to copy, skipping progress bar")
		}
		ended <- true
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
	if err != nil && cmd.ProcessState.ExitCode() > 16 {
		return 0, 0, fmt.Errorf("robocopy failed with exit code %d: %v", cmd.ProcessState.ExitCode(), err)
	}
	if args.List {
		fmt.Print(string(output))
		os.Exit(0)
	}

	var stats RobocopyStats
	err = parseRobocopyOutput(string(output), &stats)
	if err != nil {
		return 0, 0, err
	}

	return stats.Copied.Files, stats.Copied.Bytes, nil
}
