package main

import (
	"cardbackup/backup"
	"cardbackup/display"
	"cardbackup/filesystem"
	"time"

	log "github.com/sirupsen/logrus"
)

func loop() {
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

	for {
		lcd.ResetState()

		fss := <-filesystem.AfterConnect(fw)

		// Short delay before starting to user can look at screen.
		time.Sleep(5 * time.Second)

		ch := make(chan *backup.BackupProgress)
		go func() {
			for {
				lcd.SetProgress(<-ch)
			}
		}()

		if err := backup.Backup(fss, ch); err != nil {
			// TODO print error to LCD.
			log.Errorf("Backup error: %v", err)
			lcd.SetError(err)
		} else {
			log.Info("Done with backup!")
			lcd.SetProgressDone()
		}

		<-filesystem.AfterDisconnect(fw)
	}
}

func main() {
	log.Info("Start cardbackup")
	loop()
	log.Info("Quit cardbackup")
}
