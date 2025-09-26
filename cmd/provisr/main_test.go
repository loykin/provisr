package main

import (
	"testing"

	"github.com/loykin/provisr"
)

func TestBuildRoot(t *testing.T) {
	mgr := &provisr.Manager{}
	rootCmd, bind := buildRoot(mgr)

	if rootCmd == nil {
		t.Fatal("buildRoot returned nil")
	}

	if bind == nil {
		t.Fatal("buildRoot returned nil bind function")
	}

	if rootCmd.Use != "provisr" {
		t.Errorf("expected rootCmd.Use to be 'provisr', got %q", rootCmd.Use)
	}

	if len(rootCmd.Commands()) == 0 {
		t.Error("expected root command to have subcommands")
	}
}

func TestCreateRootCommand(t *testing.T) {
	globalFlags := &GlobalFlags{}

	rootCmd := createRootCommand(globalFlags)

	if rootCmd == nil {
		t.Fatal("createRootCommand returned nil")
	}

	if rootCmd.Use != "provisr" {
		t.Errorf("expected rootCmd.Use to be 'provisr', got %q", rootCmd.Use)
	}
}

func TestCreateStartCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	processFlags := &ProcessFlags{}

	startCmd := createStartCommand(cmd, processFlags)

	if startCmd == nil {
		t.Fatal("createStartCommand returned nil")
	}

	if startCmd.Use != "start" {
		t.Errorf("expected startCmd.Use to be 'start', got %q", startCmd.Use)
	}
}

func TestCreateStatusCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	processFlags := &ProcessFlags{}

	statusCmd := createStatusCommand(cmd, processFlags)

	if statusCmd == nil {
		t.Fatal("createStatusCommand returned nil")
	}

	if statusCmd.Use != "status" {
		t.Errorf("expected statusCmd.Use to be 'status', got %q", statusCmd.Use)
	}
}

func TestCreateStopCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	processFlags := &ProcessFlags{}

	stopCmd := createStopCommand(cmd, processFlags)

	if stopCmd == nil {
		t.Fatal("createStopCommand returned nil")
	}

	if stopCmd.Use != "stop" {
		t.Errorf("expected stopCmd.Use to be 'stop', got %q", stopCmd.Use)
	}
}

func TestCreateCronCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	cronFlags := &CronFlags{}

	cronCmd := createCronCommand(cmd, cronFlags)

	if cronCmd == nil {
		t.Fatal("createCronCommand returned nil")
	}

	if cronCmd.Use != "cron" {
		t.Errorf("expected cronCmd.Use to be 'cron', got %q", cronCmd.Use)
	}
}

func TestCreateGroupStartCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	groupFlags := &GroupCommandFlags{}

	groupStartCmd := createGroupStartCommand(cmd, groupFlags)

	if groupStartCmd == nil {
		t.Fatal("createGroupStartCommand returned nil")
	}

	if groupStartCmd.Use != "group-start" {
		t.Errorf("expected groupStartCmd.Use to be 'group-start', got %q", groupStartCmd.Use)
	}
}

func TestCreateGroupStopCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	groupFlags := &GroupCommandFlags{}

	groupStopCmd := createGroupStopCommand(cmd, groupFlags)

	if groupStopCmd == nil {
		t.Fatal("createGroupStopCommand returned nil")
	}

	if groupStopCmd.Use != "group-stop" {
		t.Errorf("expected groupStopCmd.Use to be 'group-stop', got %q", groupStopCmd.Use)
	}
}

func TestCreateGroupStatusCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	groupFlags := &GroupCommandFlags{}

	groupStatusCmd := createGroupStatusCommand(cmd, groupFlags)

	if groupStatusCmd == nil {
		t.Fatal("createGroupStatusCommand returned nil")
	}

	if groupStatusCmd.Use != "group-status" {
		t.Errorf("expected groupStatusCmd.Use to be 'group-status', got %q", groupStatusCmd.Use)
	}
}

func TestCreateServeCommand(t *testing.T) {
	globalFlags := &GlobalFlags{}

	serveCmd := createServeCommand(globalFlags)

	if serveCmd == nil {
		t.Fatal("createServeCommand returned nil")
	}

	if serveCmd.Use != "serve [config.toml]" {
		t.Errorf("expected serveCmd.Use to be 'serve [config.toml]', got %q", serveCmd.Use)
	}
}

func TestCreateRegisterCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	registerFlags := &RegisterFlags{}

	registerCmd := createRegisterCommand(cmd, registerFlags)

	if registerCmd == nil {
		t.Fatal("createRegisterCommand returned nil")
	}

	if registerCmd.Use != "register" {
		t.Errorf("expected registerCmd.Use to be 'register', got %q", registerCmd.Use)
	}
}

func TestCreateUnregisterCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	unregisterFlags := &UnregisterFlags{}

	unregisterCmd := createUnregisterCommand(cmd, unregisterFlags)

	if unregisterCmd == nil {
		t.Fatal("createUnregisterCommand returned nil")
	}

	if unregisterCmd.Use != "unregister" {
		t.Errorf("expected unregisterCmd.Use to be 'unregister', got %q", unregisterCmd.Use)
	}
}

func TestCreateRegisterFileCommand(t *testing.T) {
	mgr := &provisr.Manager{}
	cmd := command{mgr: mgr}
	registerFileFlags := &RegisterFileFlags{}

	registerFileCmd := createRegisterFileCommand(cmd, registerFileFlags)

	if registerFileCmd == nil {
		t.Fatal("createRegisterFileCommand returned nil")
	}

	if registerFileCmd.Use != "register-file" {
		t.Errorf("expected registerFileCmd.Use to be 'register-file', got %q", registerFileCmd.Use)
	}
}
