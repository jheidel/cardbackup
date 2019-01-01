package filesystem

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

const (
	ScanPeriod = 10 * time.Second
)

type Watcher struct {
	addl, removel chan *WatcherListener
	listeners     map[*WatcherListener]bool
	tick          <-chan time.Time

	fw *fsnotify.Watcher
}

type WatcherListener struct {
	Filesystems chan *Filesystems
	w           *Watcher
}

func NewWatcher() (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %v", err)
	}
	if err := fw.Add("/media/"); err != nil {
		return nil, fmt.Errorf("watching filesystem: %v", err)
	}
	w := &Watcher{
		addl:      make(chan *WatcherListener),
		removel:   make(chan *WatcherListener),
		listeners: make(map[*WatcherListener]bool),
		fw:        fw,
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) NewListener() *WatcherListener {
	wl := &WatcherListener{
		Filesystems: make(chan *Filesystems),
		w:           w,
	}
	w.addl <- wl
	return wl
}

func (wl *WatcherListener) Close() {
	wl.w.removel <- wl
}

func (w *Watcher) loop() {
	for {
		select {
		case wl := <-w.addl:
			w.listeners[wl] = true
			w.scan() // so it gets results right away
		case wl := <-w.removel:
			delete(w.listeners, wl)
		case _ = <-w.tick:
			w.scan()
		case e := <-w.fw.Events:
			log.Infof("Scanning filesystem due to event: %v", spew.Sdump(e))
			w.tick = time.After(1 * time.Second)
		}
	}
}

func (w *Watcher) scan() {
	r, err := Scan()
	if err != nil {
		log.Warnf("Failed filesystem scan: %v", err)
		return
	}
	for l, _ := range w.listeners {
		l.Filesystems <- r
	}

	// schedule next run
	w.tick = time.After(ScanPeriod)
}
