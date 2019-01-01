package main

import (
	"cardbackup/backup"
	"cardbackup/display"
	"cardbackup/filesystem"

	//"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

func doWork() {
	lcd, err := lcd.NewDisplay()
	if err != nil {
		panic(err)
	}
	defer lcd.Close()

	fw, err := filesystem.NewWatcher()
	if err != nil {
		panic(err)
	}

	fwl := fw.NewListener()
	defer fwl.Close()

	var fss *filesystem.Filesystems
waitStart:
	for {
		select {
		case fss = <-fwl.Filesystems:
			//spew.Dump(fss)
			if fss.Src != nil && fss.Dst != nil {
				break waitStart
			}
		}
	}

	ch := make(chan *backup.BackupProgress, 0)
	go func() {
		for _ = range ch {
			//spew.Dump(p)
		}

	}()

	if err := backup.Backup(fss, ch); err != nil {
		panic(err)
	}
	log.Info("Done!")
}

func main() {
	doWork()
	log.Info("Back in main!")
}
