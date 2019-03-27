package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"io"
)

var (
	_ fs.Handle         = &FileHandle{}
	_ fs.HandleReleaser = &FileHandle{}
	_ fs.HandleReader   = &FileHandle{}
	_ fs.HandleWriter   = &FileHandle{}
)

type FileHandle struct {
	r io.ReadWriteCloser
}

func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fh.r.Close()
}

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

func (fh *FileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	n, err := fh.r.Write(req.Data)
	if err != nil {
		return fuse.ENOENT
	}

	resp.Size = n
	return nil
}
