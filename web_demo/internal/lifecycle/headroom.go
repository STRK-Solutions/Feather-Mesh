package lifecycle

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

// Admission reserves host headroom in addition to per-container cgroups. It
// never applies the host's 20 GiB floor to a 2 GiB participant filesystem.
func VerifyHeadroom() error {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return errors.New("host memory headroom unavailable")
	}
	var total, available uint64
	for _, line := range strings.Split(string(raw), "\n") {
		var key, unit string
		var kb uint64
		if _, err := fmt.Sscanf(line, "%s %d %s", &key, &kb, &unit); err == nil && unit == "kB" {
			if key == "MemTotal:" {
				total = kb << 10
			}
			if key == "MemAvailable:" {
				available = kb << 10
			}
		}
	}
	reserve := max(uint64(2<<30), total*15/100)
	if total == 0 || available < reserve+(512<<20) {
		return errors.New("host memory headroom exhausted")
	}
	var fs unix.Statfs_t
	if err = unix.Statfs("/home/feam-service-data", &fs); err != nil {
		return err
	}
	free := uint64(fs.Bavail) * uint64(fs.Bsize)
	capacity := uint64(fs.Blocks) * uint64(fs.Bsize)
	if free < max(uint64(20<<30), capacity*15/100) {
		return errors.New("host storage headroom exhausted")
	}
	return nil
}
