package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// daemonize starts the process as a daemon in the background
func daemonize(pidFile string, logFile string) error {
	// Check if already running as daemon (child process)
	if os.Getppid() == 1 {
		// Already running as daemon
		return nil
	}

	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare command args (excluding --daemonize flag)
	var newArgs []string
	skipNext := false
	for _, arg := range os.Args[1:] {
		if skipNext {
			skipNext = false
			continue
		}

		// Skip daemonize flag
		if arg == "--daemonize" {
			continue
		}
		if arg == "--pidfile" {
			skipNext = true
			continue
		}
		if arg == "--logfile" {
			skipNext = true
			continue
		}

		newArgs = append(newArgs, arg)
	}

	// Add back pidfile and logfile if they were specified
	if pidFile != "" {
		newArgs = append(newArgs, "--pidfile", pidFile)
	}
	if logFile != "" {
		newArgs = append(newArgs, "--logfile", logFile)
	}

	// Create the child process
	// #nosec 204
	cmd := exec.Command(executable, newArgs...)

	// Set process attributes for daemon
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Redirect stdin, stdout, stderr
	cmd.Stdin = nil

	if logFile != "" {
		// Redirect stdout and stderr to log file
		// #nosec 304
		logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		cmd.Stdout = logF
		cmd.Stderr = logF
	} else {
		// Redirect to /dev/null
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

	// Start the child process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Write PID file if requested
	if pidFile != "" {
		if err := writePidFile(pidFile, cmd.Process.Pid); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
	}

	fmt.Printf("Daemon started with PID %d\n", cmd.Process.Pid)

	// Parent process exits
	os.Exit(0)
	return nil
}

// writePidFile writes the daemon PID to a file
func writePidFile(pidFile string, pid int) error {
	// #nosec 302
	f, err := os.OpenFile(pidFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(strconv.Itoa(pid))
	return err
}

// removePidFile removes the PID file
func removePidFile(pidFile string) error {
	if pidFile == "" {
		return nil
	}
	return os.Remove(pidFile)
}
