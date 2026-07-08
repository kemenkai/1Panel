package terminal

import (
	"os"
	"os/exec"
	"time"

	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/creack/pty"
	"github.com/pkg/errors"
)

var DefaultCloseSignal os.Signal = os.Interrupt

const DefaultCloseTimeout = 10 * time.Second

type LocalCommand struct {
	closeSignal  os.Signal
	closeTimeout time.Duration

	cmd *exec.Cmd
	pty *os.File
}

func NewCommand(name string, arg ...string) (*LocalCommand, error) {
	cmd := exec.Command(name, arg...)
	if term := os.Getenv("TERM"); term != "" {
		cmd.Env = append(os.Environ(), "TERM="+term)
	} else {
		cmd.Env = append(os.Environ(), "TERM=xterm")
	}
	homeDir, _ := os.UserHomeDir()
	cmd.Dir = homeDir

	pty, err := pty.Start(cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start command")
	}

	lcmd := &LocalCommand{
		closeSignal:  DefaultCloseSignal,
		closeTimeout: DefaultCloseTimeout,

		cmd: cmd,
		pty: pty,
	}

	return lcmd, nil
}

func (lcmd *LocalCommand) Read(p []byte) (n int, err error) {
	return lcmd.pty.Read(p)
}

func (lcmd *LocalCommand) Write(p []byte) (n int, err error) {
	return lcmd.pty.Write(p)
}

func (lcmd *LocalCommand) Close() error {
	if lcmd.pty != nil {
		lcmd.pty.Write([]byte{3})
		time.Sleep(50 * time.Millisecond)

		lcmd.pty.Write([]byte{4})
		time.Sleep(50 * time.Millisecond)

		lcmd.pty.Write([]byte("exit\n"))
		time.Sleep(50 * time.Millisecond)
	}
	if lcmd.cmd != nil && lcmd.cmd.Process != nil {
		_ = lcmd.cmd.Process.Signal(lcmd.closeSignal)
		time.Sleep(50 * time.Millisecond)

		if lcmd.cmd.ProcessState == nil || !lcmd.cmd.ProcessState.Exited() {
			_ = lcmd.cmd.Process.Kill()
		}
	}
	_ = lcmd.pty.Close()
	return nil
}

func (lcmd *LocalCommand) ResizeTerminal(width int, height int) error {
	return pty.Setsize(lcmd.pty, &pty.Winsize{Rows: uint16(height), Cols: uint16(width)})
}

func (lcmd *LocalCommand) Wait(quitChan chan bool) {
	if err := lcmd.cmd.Wait(); err != nil {
		global.LOG.Errorf("ssh session wait failed, err: %v", err)
		setQuit(quitChan)
	}
	setQuit(quitChan)
}
