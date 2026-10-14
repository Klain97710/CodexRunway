//go:build !windows

package grok

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// systemGrokProcessIDs lists Grok CLI executables, excluding this process.
func systemGrokProcessIDs(ctx context.Context) ([]int, error) {
	cmd := exec.CommandContext(ctx, "ps", "-axo", "pid=,comm=,args=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return parseGrokProcessIDs(out.String(), os.Getpid()), nil
}
