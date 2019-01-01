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

		// Wait for user to plug in card before continuing.
		fss := <-filesystem.AfterConnect(fw)

		// Artificial short delay before starting so user can see
		// card detected on screen.
		time.Sleep(5 * time.Second)

		ch := make(chan *backup.BackupProgress)
		go func() {
			for {
				lcd.SetProgress(<-ch)
			}
		}()

		if err := backup.Backup(fss, ch); err != nil {
			log.Errorf("Backup error: %v", err)
			lcd.SetError(err)
		} else {
			log.Info("Done with backup!")
			lcd.SetProgressDone()
		}

		// Wait for user to unplug card before continuing.
		<-filesystem.AfterDisconnect(fw)
	}
}

func main() {
	log.Info("Start cardbackup")
	loop()
}
