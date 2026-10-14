package grok

import (
	"path"
	"strconv"
	"strings"
)

// parseGrokProcessIDs reads `ps -axo pid=,comm=,args=` output.
func parseGrokProcessIDs(output string, self int) []int {
	var pids []int
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid == self {
			continue
		}
		if isGrokExecutable(fields[1], false) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// isGrokExecutable follows Swift's executable-basename rule. On Windows the
// executable suffix and path separator are part of the native file naming.
func isGrokExecutable(executable string, windows bool) bool {
	if windows {
		executable = strings.ReplaceAll(executable, `\`, "/")
	}
	name := strings.ToLower(path.Base(executable))
	return name == "grok" || (windows && name == "grok.exe")
}

func windowsGrokProcessID(pid uint32, executable string, self int) (int, bool) {
	if pid == uint32(self) || !isGrokExecutable(executable, true) {
		return 0, false
	}
	return int(pid), true
}
