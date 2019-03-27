package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/spf13/afero"
	"sync"
)

var _ fs.FS = &FileSystem{}

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
