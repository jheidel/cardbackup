package filesystem

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
)

type Filesystem struct {
	Size, Used, Available int64
	Path                  string
}

func Scan() ([]*Filesystem, error) {
	fss := []*Filesystem{}

	args := []string{
		"10", "df", "-B1",
	}

	mountpoints, err := ioutil.ReadDir("/media/")
	if err != nil {
		return fss, fmt.Errorf("failed to read mountpoints: %v", err)
	}
	if len(mountpoints) == 0 {
		// Empty!
		return fss, nil
	}
	for _, mp := range mountpoints {
		args = append(args, fmt.Sprintf("/media/%s", mp.Name()))
	}
	cmd := exec.Command("timeout", args...)
	out, err := cmd.Output()
	if err != nil {
		return fss, fmt.Errorf("filesystem scan returned error: %v", err)
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 1 {
		return fss, fmt.Errorf("unexpected df output: %v", out)
	}

	// Parse output.
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 6 {
			return fss, fmt.Errorf("unexpected df line: %v", line)
		}

		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return fss, fmt.Errorf("error parsing size from line: %v", line)
		}
		used, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return fss, fmt.Errorf("error parsing used from line: %v", line)
		}
		avail, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return fss, fmt.Errorf("error parsing avail from line: %v", line)
		}

		fs := &Filesystem{
			Size:      size,
			Used:      used,
			Available: avail,
			Path:      fields[5],
		}
		fss = append(fss, fs)
	}

	return fss, nil
}
