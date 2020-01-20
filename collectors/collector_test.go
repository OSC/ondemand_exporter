package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
)

var (
	mockedExitStatus = 0
	mockedStdout     string
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelper", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	es := strconv.Itoa(mockedExitStatus)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1",
		"STDOUT=" + mockedStdout,
		"EXIT_STATUS=" + es}
	return cmd
}

func TestExecCommandHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	fmt.Fprintf(os.Stdout, os.Getenv("STDOUT"))
	i, _ := strconv.Atoi(os.Getenv("EXIT_STATUS"))
	os.Exit(i)
}

func TestGetActivePuns(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
foo
bar`
	expPuns := []string{"foo", "bar"}
	defer func() { execCommand = exec.Command }()
	puns, _ := getActivePuns()
	if !reflect.DeepEqual(puns, expPuns) {
		t.Errorf("Expected %v, got %v", expPuns, puns)
	}
}

