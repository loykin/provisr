//go:build !windows

package detector

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"

	gopsproc "github.com/shirou/gopsutil/v4/process"
	"github.com/tklauser/go-sysconf"
)

// getProcStartUnix returns the process start time as Unix seconds using platform-native methods.
// Returns 0 when unavailable or on error.
func getProcStartUnix(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	switch runtime.GOOS {
	case "linux":
		return getProcStartUnixLinux(pid)
	default:
		p, err := gopsproc.NewProcess(int32(pid))
		if err != nil {
			return 0
		}
		ms, err := p.CreateTime()
		if err != nil || ms <= 0 {
			return 0
		}
		return ms / 1000
	}
}

func getProcStartUnixLinux(pid int) int64 {
	// Read /proc/[pid]/stat and extract starttime (field 22, in clock ticks since boot)
	statPath := "/proc/" + strconv.Itoa(pid) + "/stat"
	b, err := os.ReadFile(statPath)
	if err != nil {
		return 0
	}
	line := string(b)
	end := strings.LastIndex(line, ") ")
	if end == -1 {
		return 0
	}
	rest := strings.TrimSpace(line[end+2:])
	parts := strings.Fields(rest)
	if len(parts) < 20 {
		return 0
	}
	startTicks, err := strconv.ParseInt(parts[19], 10, 64)
	if err != nil || startTicks <= 0 {
		return 0
	}

	// Get system boot time from /proc/stat (btime)
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	var btime int64
	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		if strings.HasPrefix(text, "btime ") {
			v := strings.TrimSpace(strings.TrimPrefix(text, "btime "))
			if bt, err := strconv.ParseInt(v, 10, 64); err == nil {
				btime = bt
				break
			}
		}
	}
	if btime == 0 {
		return 0
	}

	clk, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil || clk <= 0 {
		clk = 100
	}
	return btime + (startTicks / int64(clk))
}
