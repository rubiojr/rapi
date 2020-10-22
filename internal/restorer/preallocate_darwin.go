package restorer

import (
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

func preallocateFile(wr *os.File, size int64) error {
	// try contiguous first
	fst := unix.Fstore_t{
		Flags:   unix.F_ALLOCATECONTIG | unix.F_ALLOCATEALL,
		Posmode: unix.F_PEOFPOSMODE,
		Offset:  0,
		Length:  size,
	}
	_, err := unix.FcntlInt(wr.Fd(), unix.F_PREALLOCATE, int(uintptr(unsafe.Pointer(&fst))))

	if err == nil {
		return nil
	}

	// just take preallocation in any form, but still ask for everything
	fst.Flags = unix.F_ALLOCATEALL
	_, err = unix.FcntlInt(wr.Fd(), unix.F_PREALLOCATE, int(uintptr(unsafe.Pointer(&fst))))

	// Keep struct alive until fcntl has returned
	runtime.KeepAlive(fst)

	return err
}
