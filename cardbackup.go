package main

import (
	"cardbackup/backup"
	"cardbackup/display"
	"cardbackup/filesystem"
	"time"

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

	go func() {
		lcdfw := fw.NewListener()
		for {
			lcd.SetFilesystems(<-lcdfw.Filesystems)
		}
	}()

	fwl := fw.NewListener()
	defer fwl.Close()

	fss := <-filesystem.AfterConnect(fw)

	ch := make(chan *backup.BackupProgress, 0)
	go func() {
		for {
			lcd.SetProgress(<-ch)
		}
	}()

	if err := backup.Backup(fss, ch); err != nil {
		panic(err)
	}
	log.Info("Done!")

	lcd.SetDone()

	time.Sleep(30 * time.Second)
}

func main() {
	doWork()
	log.Info("Back in main!")
}
