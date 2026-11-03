//go:build !windows

package procstat

import "time"

func memory() (private, total uint64) { return 0, 0 }

func cpuTime() time.Duration { return 0 }
