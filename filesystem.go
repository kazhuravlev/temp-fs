package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"fmt"
	"github.com/spf13/afero"
	"golang.org/x/net/context"
	"os"
	"sync"
	"time"
)

var (
	_ fs.FS         = &FileSystem{}
	_ fs.FSStatfser = &FileSystem{}
)

type FileSystem struct {
	cache *afero.Afero
	conn  *fuse.Conn
	m     *sync.RWMutex

	id2Path map[fuse.NodeID]string
	path2id map[string]fuse.NodeID
	maxID   fuse.NodeID
}

func (f *FileSystem) SetID(path string) fuse.NodeID {
	f.m.Lock()
	id, ok := f.path2id[path]
	if !ok {
		f.maxID += 1
		f.path2id[path] = f.maxID
	}
	f.m.Unlock()

	return id
}

func (f *FileSystem) GetByID(id fuse.NodeID) string {
	f.m.RLock()
	filePath, _ := f.id2Path[id]
	f.m.RUnlock()

	return filePath
}

func (f *FileSystem) Root() (fs.Node, error) {
	n := &Dir{
		fs:   f,
		path: "/",
	}
	return n, nil
}

func (f *FileSystem) Mount(path string) error {

	if err := fuse.Unmount(path); err != nil {
		return err
	}

	c, err := fuse.Mount(path)
	if err != nil {
		return err
	}
	defer c.Close()

	go f.run()

	if err := fs.Serve(c, f); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}

	return nil
}

func (f *FileSystem) Close() error {
	return f.conn.Close()
}

func (f *FileSystem) Stat(filePath string, attr *fuse.Attr) error {
	stat, err := f.cache.Stat(filePath)
	if err != nil {
		return fuse.ENOENT
	}

	if stat.IsDir() {
		attr.Mode = os.ModeDir | 0664
	} else {
		attr.Mode = 0664
	}

	now := time.Now()
	attr.Size = uint64(stat.Size())
	attr.Mtime = now
	attr.Ctime = now
	attr.Crtime = now
	return nil
}

func (f *FileSystem) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	fmt.Println("Fs.Statfs")

	resp.Bfree = 1024 * 1024 * 1024
	resp.Blocks = 1024 * 1024 * 1024
	resp.Bavail = 1024 * 1024 * 1024
	resp.Bsize = 1024 * 1024 * 1024
	resp.Ffree = 1024 * 1024 * 1024
	resp.Frsize = 3
	return nil
}

func (f *FileSystem) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timeout := 10 * time.Second

	for {
		select {
		case now := <-ticker.C:
			err := f.cache.Walk("/", func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}

				isOld := now.Sub(info.ModTime()) >= timeout
				if !isOld {
					return nil
				}

				f.cache.Remove(path)
				fmt.Println("[Removed]", path)
				return nil
			})

			if err != nil {
				fmt.Println("err", err)
				continue
			}
		}
	}
}
