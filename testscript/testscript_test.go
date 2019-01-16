// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testscript

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func printArgs() int {
	fmt.Printf("%q\n", os.Args)
	return 0
}

func exitWithStatus() int {
	n, _ := strconv.Atoi(os.Args[1])
	return n
}

func signalCatcher() int {
	// Note: won't work under Windows.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	// Create a file so that the test can know that
	// we will catch the signal.
	if err := ioutil.WriteFile("catchsignal", nil, 0666); err != nil {
		fmt.Println(err)
		return 1
	}
	<-c
	fmt.Println("caught interrupt")
	return 0
}

func TestMain(m *testing.M) {
	os.Exit(RunMain(m, map[string]func() int{
		"printargs":     printArgs,
		"status":        exitWithStatus,
		"signalcatcher": signalCatcher,
	}))
}

func TestCRLFInput(t *testing.T) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create TempDir: %v", err)
	}
	defer func() {
		os.RemoveAll(td)
	}()
	tf := filepath.Join(td, "script.txt")
	contents := []byte("exists output.txt\r\n-- output.txt --\r\noutput contents")
	if err := ioutil.WriteFile(tf, contents, 0644); err != nil {
		t.Fatalf("failed to write to %v: %v", tf, err)
	}
	t.Run("_", func(t *testing.T) {
		Run(t, Params{Dir: td})
	})
}

func TestScripts(t *testing.T) {
	// TODO set temp directory.
	Run(t, Params{
		Dir: "scripts",
		Cmds: map[string]func(ts *TestScript, neg bool, args []string){
			"setSpecialVal":    setSpecialVal,
			"ensureSpecialVal": ensureSpecialVal,
			"interrupt":        interrupt,
			"waitfile":         waitFile,
		},
	})
	// TODO check that the temp directory has been removed.
}

func setSpecialVal(ts *TestScript, neg bool, args []string) {
	ts.Setenv("SPECIALVAL", "42")
}

func ensureSpecialVal(ts *TestScript, neg bool, args []string) {
	want := "42"
	if got := ts.Getenv("SPECIALVAL"); got != want {
		ts.Fatalf("expected SPECIALVAL to be %q; got %q", want, got)
	}
}

// interrupt interrupts the current background command.
// Note that this will not work under Windows.
func interrupt(ts *TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("interrupt does not support neg")
	}
	if len(args) > 0 {
		ts.Fatalf("unexpected args found")
	}
	bg := ts.BackgroundCmds()
	if got, want := len(bg), 1; got != want {
		ts.Fatalf("unexpected background cmd count; got %d want %d", got, want)
	}
	bg[0].Process.Signal(os.Interrupt)
}

func waitFile(ts *TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("waitfile does not support neg")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: waitfile file")
	}
	path := ts.MkAbs(args[0])
	for i := 0; i < 100; i++ {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		if !os.IsNotExist(err) {
			ts.Fatalf("unexpected stat error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	ts.Fatalf("timed out waiting for %q to be created", path)
}
