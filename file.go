package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"go.uber.org/zap"
)

var (
	_ fs.Node       = &File{}
	_ fs.NodeOpener = &File{}
)

type File struct {
	fs   *FileSystem
	path string
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.fs.log.Info("File.Attr", zap.String("path", f.path))
	return f.fs.Stat(f.path, a)
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	f.fs.log.Info("File.Open", zap.String("path", f.path))

	file, err := f.fs.cache.Open(f.path)
	if err != nil {
		return nil, fuse.ENOENT
	}

	// individual entries inside a zip file are not seekable
	resp.Flags |= fuse.OpenNonSeekable
	return &FileHandle{r: file}, nil
}
