package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"fmt"
)

var (
	_ fs.Node        = &File{}
	_ fs.NodeOpener  = &File{}
	_ fs.NodeRenamer = &File{}
)

type File struct {
	fs   *FileSystem
	path string
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	return f.fs.Stat(f.path, a)
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	file, err := f.fs.cache.Open(f.path)
	if err != nil {
		return nil, fuse.ENOENT
	}

	// individual entries inside a zip file are not seekable
	resp.Flags |= fuse.OpenNonSeekable
	return &FileHandle{r: file}, nil
}

func (f *File) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	fmt.Println(req.NewName, req.NewDir)
	return nil
	//f.fs.cache.Rename(f.path, req.NewName)
}
