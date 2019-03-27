package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"fmt"
	"github.com/spf13/afero"
	"os"
	"path"
)

var (
	_ fs.Node                = &Dir{}
	_ fs.Handle              = &Dir{}
	_ fs.NodeRenamer         = &Dir{}
	_ fs.NodeCreater         = &Dir{}
	_ fs.NodeMkdirer         = &Dir{}
	_ fs.NodeRequestLookuper = &Dir{}
	_ fs.HandleReadDirAller  = &Dir{}
)

type Dir struct {
	fs   *FileSystem
	path string
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {

	return stat(d.fs, d.path, a)
}

func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	fmt.Println("Rename dir", req.NewName, d.path)

	oldPath := path.Join(d.path, req.OldName)
	newPath := path.Join(d.path, req.NewName)

	if err := d.fs.cache.Rename(oldPath, newPath); err != nil {
		return fuse.ENOENT
	}

	return nil
}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	fmt.Println("Create request", req.Name, req.Node)

	filePath := path.Join(d.path, req.Name)
	f, err := d.fs.cache.Create(filePath)
	if err != nil {
		return nil, nil, fuse.ENOENT
	}

	newFile := &File{
		fs:   d.fs,
		path: filePath,
	}

	fileHandler := &FileHandle{
		r: f,
	}

	id := d.fs.SetID(filePath)
	resp.Node = id
	return newFile, fileHandler, nil
}

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	filePath := path.Join(d.path, req.Name)
	if err := d.fs.cache.MkdirAll(filePath, os.ModeDir|0664); err != nil {
		return nil, fuse.ENOENT
	}

	newDir := &Dir{
		fs:   d.fs,
		path: filePath,
	}

	fmt.Println(req.Node, filePath)
	d.fs.SetID(filePath)

	return newDir, nil
}

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	filePath := path.Join(d.path, req.Name)

	isDir, err := afero.IsDir(d.fs.cache, filePath)
	if err != nil {
		return nil, fuse.ENOENT
	}

	if isDir {
		child := &Dir{
			fs:   d.fs,
			path: filePath,
		}
		return child, nil
	}

	child := &File{
		fs:   d.fs,
		path: filePath,
	}
	return child, nil
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	fff, err := d.fs.cache.ReadDir(d.path)
	if err != nil {
		return nil, fuse.ENOENT
	}

	var res []fuse.Dirent
	for _, f := range fff {
		de := fuse.Dirent{
			Name: f.Name(),
			Type: fuse.DT_File,
		}

		if f.IsDir() {
			de.Type = fuse.DT_Dir
		}
		res = append(res, de)
	}
	return res, nil
}
