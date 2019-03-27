package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"flag"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

// We assume the zip file contains entries for directories too.

var progName = filepath.Base(os.Args[0])

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", progName)
	fmt.Fprintf(os.Stderr, "  %s ZIP MOUNTPOINT\n", progName)
	flag.PrintDefaults()
}

var _ fs.FS = (*FileSystem)(nil)

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

func main() {
	log.SetFlags(0)
	log.SetPrefix(progName + ": ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}

	//path := flag.Arg(0)
	mountpoint := flag.Arg(0)

	f := FileSystem{
		cache:   &afero.Afero{Fs: afero.NewMemMapFs()},
		m:       new(sync.RWMutex),
		path2id: map[string]fuse.NodeID{"/": fuse.RootID},
		id2Path: map[fuse.NodeID]string{fuse.RootID: "/"},
		maxID:   fuse.RootID,
	}
	if err := f.cache.MkdirAll("/", os.ModeDir|0664); err != nil {
		log.Fatal(err)
	}

	if err := f.Mount(mountpoint); err != nil {
		log.Fatal(err)
	}
}

type Dir struct {
	fs   *FileSystem
	path string
}

var _ fs.Node = (*Dir)(nil)

func stat(fs *FileSystem, fPath string, a *fuse.Attr) error {
	stat, err := fs.cache.Stat(fPath)
	if err != nil {
		return fuse.ENOENT
	}

	if stat.IsDir() {
		a.Mode = os.ModeDir | 0664
	} else {
		a.Mode = 0664
	}

	now := time.Now()
	a.Size = uint64(stat.Size())
	a.Mtime = now
	a.Ctime = now
	a.Crtime = now
	return nil
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {

	return stat(d.fs, d.path, a)
}

var _ fs.Handle = &Dir{}

var _ fs.NodeRenamer = &Dir{}

func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	fmt.Println(req.NewName, req.NewDir, d.path, d.fs.GetByID(req.NewDir))
	return nil
	//f.fs.cache.Rename(f.path, req.NewName)
}

var _ fs.NodeCreater = &Dir{}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	fmt.Println("======================")
	filePath := path.Join(d.path, req.Name)
	fmt.Println(filePath)
	f, err := d.fs.cache.Create(filePath)
	if err != nil {
		return nil, nil, fuse.ENOENT
	}
	fmt.Println(filePath, "!!!!!!!!!!!")

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

var _ fs.NodeMkdirer = &Dir{}

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

var _ = fs.NodeRequestLookuper(&Dir{})

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

var _ = fs.HandleReadDirAller(&Dir{})

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

type File struct {
	fs   *FileSystem
	path string
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	return stat(f.fs, f.path, a)
}

var _ = fs.NodeOpener(&File{})

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	file, err := f.fs.cache.Open(f.path)
	if err != nil {
		return nil, fuse.ENOENT
	}

	// individual entries inside a zip file are not seekable
	resp.Flags |= fuse.OpenNonSeekable
	return &FileHandle{r: file}, nil
}

var _ fs.NodeRenamer = &File{}

func (f *File) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	fmt.Println(req.NewName, req.NewDir)
	return nil
	//f.fs.cache.Rename(f.path, req.NewName)
}

var _ fs.Handle = (*FileHandle)(nil)

type FileHandle struct {
	r io.ReadWriteCloser
}

var _ fs.HandleReleaser = (*FileHandle)(nil)

func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fh.r.Close()
}

var _ = fs.HandleReader(&FileHandle{})

func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// We don't actually enforce Offset to match where previous read
	// ended. Maybe we should, but that would mean'd we need to track
	// it. The kernel *should* do it for us, based on the
	// fuse.OpenNonSeekable flag.
	//
	// One exception to the above is if we fail to fully populate a
	// page cache page; a read into page cache is always page aligned.
	// Make sure we never serve a partial read, to avoid that.
	buf := make([]byte, req.Size)
	n, err := io.ReadFull(fh.r, buf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]
	return err
}

var _ fs.HandleWriter = &FileHandle{}

func (fh *FileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	n, err := fh.r.Write(req.Data)
	if err != nil {
		return fuse.ENOENT
	}

	resp.Size = n
	return nil
}
