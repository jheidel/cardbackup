package filesystem

func waitForCond(w *Watcher, f func(fs *Filesystems) bool) <-chan *Filesystems {
	c := make(chan *Filesystems)
	go func() {
		l := w.NewListener()
		defer l.Close()
		for {
			fs := <-l.Filesystems
			if f(fs) {
				c <- fs
				return
			}
		}
	}()
	return c
}

func AfterConnect(w *Watcher) <-chan *Filesystems {
	return waitForCond(w, func(fs *Filesystems) bool {
		return fs.Src != nil && fs.Dst != nil
	})
}

func AfterDisconnect(w *Watcher) <-chan *Filesystems {
	return waitForCond(w, func(fs *Filesystems) bool {
		return fs.Src == nil
	})
}
