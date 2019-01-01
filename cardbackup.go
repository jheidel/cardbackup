package main

import (
	"cardbackup/backup"
	"cardbackup/filesystem"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

func main() {
	fss, err := filesystem.Scan()
	if err != nil {
		panic(err)
	}

	spew.Dump(fss)

	if fss.Src.CompletionMarker {
		log.Info("Already done.")
		return
	}

	ch := make(chan *backup.BackupProgress, 0)
	go func() {
		for p := range ch {
			spew.Dump(p)
		}

	}()

	if err := backup.Backup(fss, ch); err != nil {
		panic(err)
	}
	log.Info("Done!")
}
