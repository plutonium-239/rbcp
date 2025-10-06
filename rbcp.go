package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// # Version information
const (
	ProgramName = "rbcp"
	Author      = "plutonium-239"
)

// : this will be injected by goreleaser at buildtime through ldflags
var (
	Version   = ""
	Commit    = ""
	BuildDate = ""
)

var (
	p *tea.Program
 	config Config
 	logger *log.Logger
 	args Args
 	root string
 	dest string
 	files []string
)

type Args struct {
	Paths            []string `arg:"positional, required" placeholder:"SRC DEST"`
	Mir              bool     `arg:"-m" help:"Convenience argument to specify /MIR to robocopy"`
	List             bool     `arg:"-l" help:"Only list files that would be copied. Similar to a 'dry-run' "`
	PreserveExitCode bool     `arg:"-p,--preserve-exitcode" help:"Always return the error code given by robocopy. By default, exit with code 0 on success and passthrough on copy failures."`
	Insane           bool     `help:"Don't set sane defaults (currently sets #retries to 2 and timeout between them to 1 sec."`
	OtherArgs        []string `arg:"-[,--passthrough" help:"All other arguments to be passed directly to robocopy."`
	// !!! DISABLE IN PROD
	Profile bool
}

func (args Args) Description() string {
	b := args.Version() + "\n"
	b += "\nrbcp is a compact wrapper around robocopy, aiming to modernize the output while preserving the robustness of this time tested tool."
	b += "\nAll other arguments are passed directly to robocopy."
	return b
}

func (Args) Version() string {
	// impStyle will never be defined as GetConfig is never called in help/version text
	impStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(defaultConfig().Theme.ColorPrimary)).Bold(true)
	return impStyle.Render(ProgramName+" version "+Version) + "\n" +
		"Commit: " + Commit + "\n" +
		"Built: " + BuildDate
}

// # Setup logging, config
func setup() int {
	lvl := "warn"
	if envLoglvl := os.Getenv("LOGLEVEL"); envLoglvl != "" {
		lvl = envLoglvl
	}
	if loglvl, err := log.ParseLevel(lvl); err == nil {
		log.SetLevel(loglvl)
		logger.SetLevel(loglvl)
	}

	initWidth := 80
	if envColumns := os.Getenv("COLUMNS"); envColumns != "" {
		if i, err := strconv.Atoi(envColumns); err == nil {
			initWidth = i
		}
	}

	config = GetConfig()

	styles := log.DefaultStyles()
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		Background(lipgloss.Color(config.Theme.ColorError)).Foreground(lipgloss.Color("#fff")).
		SetString("ERROR").Padding(0, 1).Bold(true)
	logger.SetStyles(styles)
	return initWidth
}

// # Parse arguments (break Paths into Src and Dest, perform expansions, build arguments for robocopy)
func parseArgs() {
	if args.Profile {
		os.MkdirAll("prof/", os.ModeDir)
		runtime.SetBlockProfileRate(1)
		t_now := time.Now().Format("2006-01-02 15.04.05")
		f, err := os.Create("prof/" + t_now + ".pprof")
		f2, err2 := os.Create("prof/block_" + t_now + ".pprof")
		if err != nil || err2 != nil {
			logger.Fatalf("could not create CPU profile: %v", err)
		}
		defer f.Close()
		defer f2.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			logger.Fatalf("could not start CPU profile: %v", err)
		}
		pprof.Lookup("block").WriteTo(f2, 0)
		defer pprof.StopCPUProfile()
	}

	// stores the positions of files in args.Paths
	files = make([]string, 0)

	if len(args.Paths) < 2 {
		logger.Fatal("No destination specified")
	}
	dest = args.Paths[len(args.Paths)-1]

	// : bash ./{a,b} brace expansion syntax
	cfg := &expand.Config{
		// do not expand env vars or do cmd/proc substitution
		Env: nil,
		CmdSubst: nil,
		ProcSubst: nil,
		ReadDir2: os.ReadDir,
	}
	parser := syntax.NewParser()

	// : multiple files
	for _, srcf := range args.Paths[:len(args.Paths)-1] {
		word, err := parser.Document(strings.NewReader(srcf))
		if err != nil {
			logger.Fatalf("Cannot parse syntax: %v", srcf)
		}
		fields, err := expand.Fields(cfg, word)
		if err != nil {
			logger.Fatalf("Invalid path syntax: %v", srcf)
		}
		logger.Infof("Expanded %v to %v", srcf, fields)
		files = append(files, fields...)
	}
	for i, srcf := range files {
		_, err := os.Stat(srcf)
		if err == nil {
			p, f := filepath.Split(srcf)
			if p == "" {
				logger.Debugf("Empty path, setting to ./")
				p = "./"
			}
			if root == "" {
				root = p
			}
			if root != p {
				logger.Warnf("File passed not in root dir(%v): %v", root, srcf)
			}
			logger.Infof("Detected file %v and broke into %v and %v", srcf, p, f)
			// args.Src[i] = p
			// slices.Insert(args.OtherArgs, 0, f)
			files[i] = f
		} else {
			logger.Errorf(errorStyle.Render("The file trying to be copied does not exist.\n%v"), err.Error())
			os.Exit(1)
		}
	}

}

// # builds arguments for robocopy based on args. no side effects.
func buildRobocopyArgs() []string {
	out := []string{root, dest}
	out = slices.Concat(out, files, args.OtherArgs)
	// out = append(out, args.OtherArgs...)
	if args.Mir {
		out = append(out, "/MIR")
	}
	logger.Infof("Starting robocopy with arguments: %v", out)

	if !args.List {
		// Add our output formatting flags
		notAllowed := []string{"/bytes", "/np", "/njh", "/njs", "/ndl", "/nfl", "/ns"}
		slices.DeleteFunc(out, func(e string) bool {
			return slices.Contains(notAllowed, strings.ToLower(e))
		})
		// let user log if wanted, but we need output to function so tee it
		for _, e := range out {
			e = strings.ToLower(e)
			if strings.HasPrefix(e, "/log") || strings.HasPrefix(e, "/unilog") {
				out = append(out, "/tee")
			}
		}
		out = append(out, "/NJH", "/NDL", "/BYTES")
		if !args.Insane {
			out = append(out, "/R:2", "/W:1")
		}
	} else {
		fmt.Println()
	}
	return out
}

func main() {
	logger = log.New(os.Stderr)

	// : Argument parsing and applying effects
	arg.MustParse(&args)

	initWidth := setup()
	startTime := time.Now()
	parseArgs()

	arrow := pathStyle.Italic(false).Render(" --> ")
	if config.UseNerdFontArrow {
		arrow = pathStyle.Italic(false).Render(" ─── ")
	}
	fmt.Println(lipgloss.PlaceHorizontal(initWidth, lipgloss.Center,
		pathStyle.Render(root+"["+strings.Join(files, ",")+"]")+arrow+pathStyle.Render(args.Paths[len(args.Paths)-1])))

	rbarglist := buildRobocopyArgs()

	// : Dummy list-only run to get an overview of total
	// if args.List is passed the program terminates inside this
	totalFiles, totalBytes, err := getTotalCounts()
	if err != nil {
		logger.Fatalf("Error getting total counts: %v", err)
	}
	logger.Infof("Total to copy: %d files, %s\n", totalFiles, formatByteValue(totalBytes))

	// : Init TUI and  start robocopy
	m := model{
		progress: progress.New(
			progress.WithGradient(config.Theme.ColorProgress[0], config.Theme.ColorProgress[1]),
			// progress.WithSpringOptions(40, 1),
		),
		totalFiles: totalFiles,
		totalBytes: totalBytes,
		totalWidth: initWidth,
	}
	p = tea.NewProgram(m)

	var stats RobocopyStats
	// this apparently makes a 0-memory channel
	ended := make(chan struct{})
	forceQuit := make(chan struct{})
	go func() {
		if totalBytes > 0 {
			// returns after TUI exit
			t, err := p.Run()
			if err != nil {
				logger.Fatal("error running program:", err)
				os.Exit(1)
			}
			m = t.(model)
			if m.ForceQuit {
				forceQuit <- struct{}{}
			}
		} else {
			logger.Info("Nothing to copy, skipping progress bar")
		}
		ended <- struct{}{}
		// logger.Warnf("program is over, waiting for summary")
	}()

	robocopyStart := time.Now()
	var robocopyEnd time.Time
	if totalBytes > 0 {
		stats, err = runRobocopy(rbarglist, forceQuit)
		if err != nil {
			logger.Fatalf("Error: %v", err)
		}
		// logger.Debugf("Killed")
		robocopyEnd = time.Now()
		// p.Send(tea.Quit())
		p.Wait()
	}

	<-ended
	// : Display summary
	logger.Infof("Robocopy took %v", robocopyEnd.Sub(robocopyStart))
	logger.Infof("Waited for %v", time.Since(robocopyEnd))
	displaySummary(stats)

	timeTaken := time.Since(startTime)
	logger.Infof("Whole program took %v", timeTaken)

	if args.PreserveExitCode || stats.ExitCode >= 8 {
		// Exit with the same code as robocopy
		os.Exit(stats.ExitCode)
	}
}

func runRobocopy(args []string, forceQuit chan struct{}) (RobocopyStats, error) {
	var stats RobocopyStats

	// Start timing
	startTime := time.Now()

	// Run robocopy and capture output
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "robocopy", args...)
	logger.Debugf("Starting command %v", cmd)
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

	// no need to wait for both here - only one path can be true.
	// also, don't return if forceQuit - can still display stats (if any)
	select {
	case <-forceQuit:
		cmd.Cancel()
	case <-ended:
	}
	cmd.Wait()
	// Calculate duration
	endTime := time.Now()
	stats.Duration = endTime.Sub(startTime)
	stats.ExitCode = cmd.ProcessState.ExitCode()
	logger.Infof("parsing took %v", parsingTime)
	logger.Infof("Waited after cmd exit for parsing for %v", time.Since(endTime))

	logger.Debugf("%+v", stats)
	// Non-fatal error handling (robocopy uses exit codes for normal operations)
	if err != nil && stats.ExitCode > 16 {
		return stats, fmt.Errorf("robocopy failed with exit code %d: %v", stats.ExitCode, err)
	}

	return stats, nil
}

// getTotalCounts runs robocopy in list-only mode to get total files and bytes
func getTotalCounts() (int, int64, error) {
	// Clone args and add /L to make it "list only" mode
	rbargs := buildRobocopyArgs()
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
	logger.Debugf("%+v", stats)

	return stats.Copied.Files, stats.Copied.Bytes, nil
}
