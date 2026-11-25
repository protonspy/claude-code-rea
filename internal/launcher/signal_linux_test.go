//go:build linux

package launcher

import (
	"bytes"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestAnInterruptStopsNeitherTheLauncherNorIsIgnoredByOpenCode checks R6.4 on Linux: after
// ignoreInterrupts the launcher survives SIGINT, and a child it starts has SIGINT at its
// default disposition, not ignored.
func TestAnInterruptStopsNeitherTheLauncherNorIsIgnoredByOpenCode(t *testing.T) {
	if os.Getenv("REA_LAUNCHER_SIGIGN") != "" {
		return
	}
	ignoreInterrupts()
	defer signal.Reset(os.Interrupt)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // the test process is still here to go on

	var out bytes.Buffer
	code, err := runCommand(t.Context(), Command{
		Path:   os.Args[0],
		Args:   []string{"-test.run=TestHelperSigIgn"},
		Env:    append(os.Environ(), "REA_LAUNCHER_SIGIGN=1"),
		Stdout: &out,
	})
	if err != nil || code != 0 {
		t.Fatalf("child: code %d, err %v", code, err)
	}
	mask, err := strconv.ParseUint(strings.TrimSpace(out.String()), 16, 64)
	if err != nil {
		t.Fatalf("child printed %q", out.String())
	}
	if mask&(1<<(uint(syscall.SIGINT)-1)) != 0 {
		t.Errorf("the child starts with SIGINT ignored (SigIgn %x)", mask)
	}
}

// TestHelperSigIgn prints the signals its process ignores, from /proc/self/status.
func TestHelperSigIgn(t *testing.T) {
	if os.Getenv("REA_LAUNCHER_SIGIGN") == "" {
		return
	}
	status, _ := os.ReadFile("/proc/self/status")
	for _, line := range strings.Split(string(status), "\n") {
		if v, ok := strings.CutPrefix(line, "SigIgn:"); ok {
			os.Stdout.WriteString(strings.TrimSpace(v) + "\n")
		}
	}
	os.Exit(0)
}

func TestADeathBySignalExitsWith128PlusTheSignal(t *testing.T) {
	if os.Getenv("REA_LAUNCHER_SIGTERM") != "" {
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
		time.Sleep(time.Second)
		os.Exit(0)
	}
	code, err := runCommand(t.Context(), Command{
		Path: os.Args[0],
		Args: []string{"-test.run=TestADeathBySignalExitsWith128PlusTheSignal"},
		Env:  append(os.Environ(), "REA_LAUNCHER_SIGTERM=1"),
	})
	if err != nil || code != 128+int(syscall.SIGTERM) {
		t.Errorf("code %d, err %v, want %d", code, err, 128+int(syscall.SIGTERM))
	}
}
