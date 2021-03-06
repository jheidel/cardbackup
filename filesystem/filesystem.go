package filesystem

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	MarkerFile = "backup-marker.txt"

	// TODO: Use a better heuristic so this isn't needed.
	DstThreshBytes = 750 << 30
)

type Filesystem struct {
	Size, Used, Available int64
	Path                  string
}

func (fs *Filesystem) WriteSuccessFile(relpath string) error {
	ts := time.Now().Format(time.RFC3339)
	v := fmt.Sprintf("Backup of these files completed on %v\n", ts)
	mp := path.Join(fs.Path, relpath)
	if err := ioutil.WriteFile(mp, []byte(v), 0660); err != nil {
		return err
	}
	return nil
}

func (fs *Filesystem) RemountWritable(writable bool) error {
	wc := "remount,ro"
	if writable {
		wc = "remount,rw"
	}
	_, err := exec.Command("sudo", "mount", "-o", wc, fs.Path).Output()
	if err != nil {
		return fmt.Errorf("remount writable %v: %v", writable, err)
	}
	return nil
}

func scanAll() ([]*Filesystem, error) {
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
		if strings.TrimSpace(line) == "" || len(fields) == 0 {
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
		if fs.Path == "/" || fs.Path == "/media" || fs.Path == "/media/" {
			continue
		}
		fss = append(fss, fs)
	}

	return fss, nil
}

type Filesystems struct {
	Dst *Filesystem
	Src *Filesystem
}

func Scan() (*Filesystems, error) {
	fss, err := scanAll()
	if err != nil {
		return nil, err
	}

	r := &Filesystems{}
	// Heuristic: select one filesystem as destination and one as source based on size
	// TODO: Improve hard drive detection.
	for _, fs := range fss {
		if fs.Size >= DstThreshBytes {
			if r.Dst == nil {
				r.Dst = fs
			} else {
				return nil, fmt.Errorf("Multiple filesystems above destination threshold")
			}
		} else {
			if r.Src == nil {
				r.Src = fs
			} else {
				return nil, fmt.Errorf("Multiple filesystems below destination threshold")
			}
		}
	}
	return r, nil
}
