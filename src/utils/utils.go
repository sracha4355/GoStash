package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/barweiss/go-tuple"
)

type Fatal struct{}
type Info struct{}
type Debug struct{}

func WaitXSeconds(X int) {
	time.Sleep(time.Duration(X) * time.Second)
}

// ---- Unit test this, VERY IMPORTANT
func SplitWork(pages tuple.T2[int, int], batches int) ([]tuple.T2[int, int], error) {
	start, end := pages.V1, pages.V2
	if start < 0 || end < 0 || start > end {
		return nil, fmt.Errorf("invalid page range tuple passed to SplitWork")
	}
	if start == end {
		return []tuple.T2[int, int]{tuple.New2(start, end)}, nil
	}
	total := end - start + 1
	size := total / batches
	remainder := total % batches
	result := make([]tuple.T2[int, int], 0, batches)

	if size == 0 {
		for i := start; i <= end; i++ {
			result = append(result, tuple.New2(i, i))
		}
		return result, nil
	}

	current := start
	for i := 0; i < batches; i++ {
		batchSize := size
		if remainder > 0 {
			batchSize++
			remainder--
		}
		batchEnd := current + batchSize - 1
		if batchEnd > end {
			batchEnd = end
		}
		result = append(
			result,
			tuple.T2[int, int]{V1: current, V2: batchEnd},
		)
		current = batchEnd + 1
	}
	return result, nil
}

func LogWithContext(level interface{}, format string, args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1) // 1 means caller of this function
	funcName := runtime.FuncForPC(pc).Name()
	msg := fmt.Sprintf(format, args...)
	switch level.(type) {
	case Debug:
		log.Printf("DEBUG[%s]:%d %s", funcName, line, msg)
	case Fatal:
		log.Fatalf("FATAL[%s]:%d %s", funcName, line, msg)
	default:
		log.Printf("INFO[%s]:%d %s", funcName, line, msg)
	}
}

func Exists(path string) (exists bool, isDir bool, err error) {
	fi, err := os.Stat(path) // follows symlinks
	if err == nil {
		return true, fi.IsDir(), nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, false, nil
	}
	// other error (permission, I/O)
	return false, false, err
}

func AssertFilePerms(f *os.File, permMask os.FileMode) error {
	if f == nil {
		return fmt.Errorf("AssertFilePerms: nil *os.File provided")
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("AssertFilePerms: could not stat file %q: %w", f.Name(), err)
	}
	if fileInfo.Mode().Perm() == permMask {
		return nil
	}
	return fmt.Errorf(
		"AssertFilePerms: permissions mismatch for %q: got %04o, want %04o",
		f.Name(), uint32(fileInfo.Mode().Perm()), uint32(permMask),
	)
}

func TrySendError(__errChan__ chan<- error, __err__ error, doLog bool) {
	select {
	case __errChan__ <- __err__:
	default:
		if doLog {
			LogWithContext(Fatal{}, "%v", __err__)
		}
	}
}
