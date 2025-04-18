package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type langOption interface {
	cmd() string
	args() []string
	ext() string
}

func getOptions(lang string) langOption {
	switch lang {
	case "python":
		return &pythonOption{}
	case "python3":
		return &python3Option{}
	case "go":
		return &goOption{}
	default:
		panic("unsupported language:" + lang)
	}
}

type pythonOption struct{}
type python3Option struct {
	pythonOption
}

func (o *python3Option) cmd() string {
	return "python3"
}

func (o *pythonOption) cmd() string {
	return "python"
}
func (o *pythonOption) args() []string {
	return []string{}
}
func (o *pythonOption) ext() string {
	return ".py"
}

type goOption struct{}

func (o *goOption) cmd() string {
	return "go"
}
func (o *goOption) args() []string {
	return []string{"run"}
}
func (o *goOption) ext() string {
	return ".go"
}

var startTime = time.Now()

func main() {
	events := make(chan bool, 10)
	callback := func() {
		events <- true
	}
	lang := os.Getenv("GORUN_LANG")
	if lang == "" {
		lang = "go"
	}
	dir := os.Getenv("GORUN_SCAN_DIR")
	if dir == "" {
		dir = "."
	}
	opt := getOptions(lang)
	go scanChanges(dir, opt, callback)

	var runCmd *exec.Cmd
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		if runCmd != nil {
			fmt.Println("start gracefully stop")
			syscall.Kill(-runCmd.Process.Pid, syscall.SIGTERM)
			gracefulTimeout := time.NewTimer(time.Second * 10).C
			exit := make(chan bool, 1)
			go func() {
				runCmd.Wait()
				exit <- true
			}()
			select {
			case <-exit:
				fmt.Println("gracefully stop successfully")
			case <-gracefulTimeout:
				fmt.Println("gracefully stop failed, force exit")
				syscall.Kill(-runCmd.Process.Pid, syscall.SIGKILL)
				time.Sleep(time.Millisecond * 100)
			}
		}
		os.Exit(0)
	}()

	var runFunc = func() {
		if runCmd != nil {
			syscall.Kill(-runCmd.Process.Pid, syscall.SIGKILL)
			time.Sleep(time.Millisecond * 100)
		}
		args := opt.args()
		args = append(args, os.Args[1:]...)
		runCmd = exec.Command(opt.cmd(), args...)
		runCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		runCmd.Stdin = os.Stdin
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		runCmd.Env = os.Environ()
		go func() {
			err := runCmd.Run()
			if err != nil && !strings.Contains(err.Error(), ": killed") {
				fmt.Println("Run process failed", err)
			}
		}()
	}
	runFunc()
	for range events {
		func() {
			for {
				select {
				case <-events:
				default:
					return
				}
			}
		}()
		fmt.Println("rebuilding...")
		runFunc()
	}
}

func scanChanges(watchPath string, opt langOption, callback func()) {
	skipDIRs := map[string]bool{
		".git": true, ".venv": true,
	}
	if s := os.Getenv("GORUN_SKIP_DIRS"); s != "" {
		for _, x := range strings.Split(s, ":") {
			skipDIRs[x] = true
		}
	}
	allFiles := os.Getenv("GORUN_ALL_FILES") == "1"
	for {
		filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
			if skipDIRs[path] {
				return filepath.SkipDir
			}

			// ignore hidden files
			if filepath.Base(path)[0] == '.' {
				return nil
			}

			if (allFiles || filepath.Ext(path) == opt.ext()) && info.ModTime().After(startTime) {
				callback()
				startTime = time.Now()
				return errors.New("done")
			}

			return nil
		})
		time.Sleep(500 * time.Millisecond)
	}
}
