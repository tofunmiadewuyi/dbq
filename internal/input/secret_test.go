package input

import (
	"os"
	"os/exec"
	"testing"
)

func TestAskSecretRejectsNonTerminalInput(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	previous := os.Stdin
	os.Stdin = read
	defer func() { os.Stdin = previous }()
	if _, err := AskSecret("Secret: ", func(value string) error { return ValidateField("Secret", value) }, "saved-secret"); err == nil {
		t.Fatal("secret prompt accepted a non-terminal fallback")
	}
}

// Child process fixture for the real terminal test below.
func TestSecretPromptProcess(t *testing.T) {
	scenario := os.Getenv("DBQ_TEST_PROMPT_SCENARIO")
	if scenario == "" {
		t.Skip("terminal subprocess fixture")
	}
	existing, want := "", "  new-secret  "
	if scenario == "keep" {
		existing, want = "saved-secret", "saved-secret"
	}
	got, err := AskSecret("Secret: ", func(value string) error { return ValidateField("Secret", value) }, existing)
	if err != nil || got != want {
		t.Fatal("terminal secret read failed or changed the value")
	}
}

func TestAskSecretTerminalDoesNotEchoOrDisplayDefaults(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("PTY integration requires Python")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := `
import os, pty, select, subprocess, sys, termios, time
for scenario in ('new', 'keep'):
    master, slave = pty.openpty()
    env = dict(os.environ, DBQ_TEST_PROMPT_SCENARIO=scenario)
    proc = subprocess.Popen([sys.argv[1], '-test.run=^TestSecretPromptProcess$'], stdin=slave, stdout=slave, stderr=slave, env=env)
    output = b''
    deadline = time.monotonic() + 10
    try:
        while b': ' not in output:
            if time.monotonic() > deadline: raise RuntimeError('secret prompt did not appear')
            if select.select([master], [], [], .1)[0]: output += os.read(master, 4096)
        while termios.tcgetattr(slave)[3] & termios.ECHO:
            if time.monotonic() > deadline: raise RuntimeError('terminal echo was not disabled')
            time.sleep(.01)
        os.write(master, b'  new-secret  \n' if scenario == 'new' else b'\n')
        while proc.poll() is None:
            if time.monotonic() > deadline: raise RuntimeError('secret prompt hung')
            if select.select([master], [], [], .1)[0]: output += os.read(master, 4096)
        while select.select([master], [], [], .05)[0]: output += os.read(master, 4096)
        if proc.returncode != 0: raise RuntimeError('secret value was not read correctly')
        if b'new-secret' in output or b'saved-secret' in output: raise RuntimeError('terminal output exposed a secret')
        if not termios.tcgetattr(slave)[3] & termios.ECHO: raise RuntimeError('terminal echo was not restored')
    finally:
        if proc.poll() is None: proc.kill(); proc.wait()
        os.close(master); os.close(slave)
`
	if output, err := exec.Command(python, "-c", script, binary).CombinedOutput(); err != nil {
		t.Fatalf("terminal prompt validation failed: %v\n%s", err, output)
	}
}
