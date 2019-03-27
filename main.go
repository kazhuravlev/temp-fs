package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"flag"
	"fmt"
	"github.com/spf13/afero"
	"log"
	"os"
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
