package provision

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type lineSink struct {
	pending []byte
	capture bool
	state   *commandOutput
}
type commandOutput struct {
	mu     sync.Mutex
	out    []byte
	onLine func(string)
}

func (s *lineSink) Write(p []byte) (int, error) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if s.capture {
		s.state.out = append(s.state.out, p...)
		if len(s.state.out) > 4<<20 {
			s.state.out = append([]byte(nil), s.state.out[len(s.state.out)-(4<<20):]...)
		}
	}
	s.pending = append(s.pending, p...)
	for {
		index := bytes.IndexByte(s.pending, '\n')
		if index < 0 {
			break
		}
		if s.state.onLine != nil {
			s.state.onLine(string(s.pending[:index]))
		}
		s.pending = s.pending[index+1:]
	}
	if len(s.pending) > 1<<20 {
		s.pending = s.pending[len(s.pending)-(1<<20):]
	}
	return len(p), nil
}
func (s *lineSink) flush() {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if len(s.pending) > 0 && s.state.onLine != nil {
		s.state.onLine(string(s.pending))
	}
	s.pending = nil
}
func runCommand(ctx context.Context, path string, args []string, dir string, onLine func(string)) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	cleanup := configureCommand(cmd)
	defer cleanup()
	cmd.WaitDelay = 3 * time.Second
	state := &commandOutput{onLine: onLine}
	stdout := &lineSink{state: state, capture: true}
	stderr := &lineSink{state: state}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	stdout.flush()
	stderr.flush()
	if ctx.Err() != nil {
		return state.out, ctx.Err()
	}
	if err != nil {
		return state.out, fmt.Errorf("%s: %w", path, err)
	}
	return state.out, nil
}
