package backup

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"cardbackup/filesystem"

	log "github.com/sirupsen/logrus"
)

const (
	DirFormat = "Backup-2006-01-02T15-04-05Z"
)

type BackupProgress struct {
	Percent   int32
	BytesSent int64
	Rate      string
	Elapsed   time.Duration
	Remaining time.Duration
}

func cleanLine(token []byte) []byte {
	return []byte(strings.TrimSpace(string(token)))
}

// Scans for carriage returns to handle rsync output.
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	nb := bytes.IndexByte(data, '\r')
	nn := bytes.IndexByte(data, '\n')
	i := nb
	if nn > i {
		i = nn
	}
	if i >= 0 {
		return i + 1, cleanLine(data[0:i]), nil
	}
	if atEOF {
		return len(data), cleanLine(data), nil
	}
	return 0, nil, nil
}

func buildProgress(start, now time.Time, line string) *BackupProgress {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil
	}

	pi := strings.IndexByte(fields[1], '%')
	if pi == -1 {
		return nil
	}
	percent, err := strconv.ParseInt(fields[1][0:pi], 10, 32)
	if err != nil {
		return nil
	}

	if percent < 0 || percent > 100 {
		return nil
	}

	sent, err := strconv.ParseInt(strings.Replace(fields[0], ",", "", -1), 10, 64)
	if err != nil {
		return nil
	}

	elapsed := now.Sub(start)
	estTotal := time.Duration(0)
	if percent > 0 {
		estTotal = time.Duration(elapsed.Nanoseconds() * 100 / percent)
	}

	return &BackupProgress{
		Percent:   int32(percent),
		BytesSent: sent,
		Rate:      fields[2],
		Elapsed:   elapsed,
		Remaining: estTotal - elapsed,
	}
}

func Backup(fs *filesystem.Filesystems, progress chan<- *BackupProgress) error {
	if fs.Src == nil || fs.Dst == nil {
		return fmt.Errorf("Missing backup source or destination")
	}

	start := time.Now()
	dirname := start.Format(DirFormat)
	// Trailing slash needed here to avoid new subdir.
	src := fs.Src.Path + "/"
	dst := path.Join(fs.Dst.Path, dirname)
	log.Infof("Starting backup from %v to %v", src, dst)

	cmd := exec.Command("rsync", "-av", "--info=progress2", "--no-i-r", src, dst)

	if err := os.Mkdir(dst, 0777); err != nil {
		return fmt.Errorf("creating log dir: %v", err)
	}
	logfile, err := os.Create(path.Join(dst, "rsync.log"))
	if err != nil {
		return fmt.Errorf("opening log: %v", err)
	}
	logw := bufio.NewWriterSize(logfile, 4096)
	logl := &sync.Mutex{}

	flusher := time.NewTicker(10 * time.Second)
	go func() {
		for _ = range flusher.C {
			logl.Lock()
			logw.Flush()
			logl.Unlock()
		}
	}()

	progress <- &BackupProgress{}

	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stdout := bufio.NewScanner(r)
	stdout.Split(scanLines)
	go func() {
		for stdout.Scan() {
			if p := buildProgress(start, time.Now(), stdout.Text()); p != nil {
				progress <- p
			} else {
				v := strings.TrimSpace(stdout.Text())
				if len(v) > 0 {
					logl.Lock()
					logw.WriteString(v + "\n")
					logl.Unlock()
				}
			}
		}
	}()

	r, err = cmd.StderrPipe()
	if err != nil {
		return err
	}
	stderr := bufio.NewScanner(r)
	go func() {
		for stderr.Scan() {
			logl.Lock()
			logw.WriteString(stderr.Text() + "\n")
			logl.Unlock()
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start backup: %v", err)
	}

	status := cmd.Wait()
	log.Infof("rsync command completed with status %v", err)

	flusher.Stop()

	logl.Lock()
	defer logl.Unlock()

	log.Info("flushing log file")
	if err := logw.Flush(); err != nil {
		return fmt.Errorf("flushing log: %v", err)
	}
	if err := logfile.Close(); err != nil {
		return fmt.Errorf("closing log: %v", err)
	}

	if status != nil {
		return fmt.Errorf("backup failed: %v", status)
	}

	// Mark as done on source filesystem.
	log.Info("writing completion marker")
	if err := fs.Src.WriteCompletionMarker(); err != nil {
		return fmt.Errorf("writing completion marker: %v", err)
	}

	return nil
}
