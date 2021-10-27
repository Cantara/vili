package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"

	"k8s.io/utils/inotify"
)

func tailFile(path string, ctx context.Context) (lineChan chan []byte, err error) {
	parts := strings.Split(path, "/")
	folder := strings.Join(parts[:len(parts)-1], "/")
	watcher, err := inotify.NewWatcher()
	if err != nil {
		return
	}
	err = watcher.AddWatch(folder, inotify.InMovedFrom|inotify.InCreate)
	if err != nil {
		watcher.Close()
		return
	}
	lineChan = make(chan []byte, 20)
	go func() {
		defer watcher.Close()
		defer watcher.RemoveWatch(folder)
		r, file, err := newFileWatcherAndReader(path, watcher)
		if err == nil {
			defer watcher.RemoveWatch(path)
			defer file.Close()
		}
		err = nil
		for {
			select {
			case ev := <-watcher.Event:
				if ev.Mask != inotify.InModify {
					if ev.Name != path {
						continue
					}
					if ev.Mask == inotify.InMovedFrom {
						watcher.RemoveWatch(path)
						file.Close()
						continue
					}
					if ev.Mask == inotify.InCreate {
						r, file, err = newFileWatcherAndReader(path, watcher)
						if err == nil {
							defer watcher.RemoveWatch(path)
							defer file.Close()
						}
						continue
					}
					continue
				}
				stat, err := file.Stat()
				if err != nil {
					continue
				}
				if stat.Size() == 0 {
					file.Close()
					file, err = os.Open(path)
					if err != nil {
						log.Println(err)
						continue
					}
					r = bufio.NewReader(file)
					continue
				}
				for line := []byte{}; err == nil; line, err = r.ReadBytes('\n') {
					if len(line) > 0 {
						lineChan <- line
					}
				}
			case err := <-watcher.Error:
				log.Println("event error:", err)
			case <-ctx.Done():
				break
			}
		}
		close(lineChan)
	}()
	return
}

func newFileWatcherAndReader(path string, watcher *inotify.Watcher) (reader *bufio.Reader, file *os.File, err error) {
	err = watcher.AddWatch(path, inotify.InModify)
	if err != nil {
		return
	}
	file, err = os.Open(path)
	if err != nil {
		return
	}
	reader = bufio.NewReader(file)
	return
}
