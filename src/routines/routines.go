package routines

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/sracha4355/psp/src/contracts"
	"github.com/sracha4355/psp/src/utils"
)

// ---- New API in progress ---- will need to refactor tests
type AsyncFileWriterConfig struct {
	WhenToFlush   int
	FlushInterval time.Duration
	SupressLogs   bool
	Writer        *io.Writer
}
type AsyncFileWriter[T contracts.Serializable[T]] struct {
	wg           *sync.WaitGroup
	InputChannel <-chan T
	ErrorChannel chan<- error
	DoneChannel  <-chan struct{}
	Ctx          context.Context
	Config       AsyncFileWriterConfig
}

func (w *AsyncFileWriter[T]) NewAsyncFileWriter(
	__when_to_flush__ int,
	__flush_interval__ time.Duration,
	__supress_logs__ bool,
	__writer__ *io.Writer,
	__input_channel__ <-chan T,
	__error_channel__ chan<- error,
	__done_channel__ <-chan struct{},
	__ctx__ context.Context,
	__waitgroup__ *sync.WaitGroup,
) {
	w.wg = __waitgroup__
	w.InputChannel = __input_channel__
	w.ErrorChannel = __error_channel__
	w.DoneChannel = __done_channel__
	w.Ctx = __ctx__
	w.Config = AsyncFileWriterConfig{
		WhenToFlush:   __when_to_flush__,
		FlushInterval: __flush_interval__,
		SupressLogs:   __supress_logs__,
		Writer:        __writer__,
	}
}

func (afw *AsyncFileWriter[T]) Run() {
	defer func() {
		afw.wg.Done()
	}()
	var internals *AsyncFileWriterConfig = &afw.Config
	if internals.WhenToFlush <= 0 {
		internals.WhenToFlush = 100
	}

	// -- pass value for buz size later
	w := bufio.NewWriter(*internals.Writer)
	ticker := time.NewTicker(internals.FlushInterval)
	defer func() {
		_ = w.Flush()
	}()

	for {
		select {
		case <-afw.DoneChannel:
			if errorWhileDraining := afw.__drain__(w); errorWhileDraining != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileDraining, internals.SupressLogs)
			}
			if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
			}
			goto done
		case <-afw.Ctx.Done():
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Context cancellation occurred:%v", afw.Ctx.Err())
			}
			goto done
		case item, ok := <-afw.InputChannel:
			if !ok {
				if !internals.SupressLogs {
					utils.LogWithContext(utils.Info{}, "Input channel closed --- exiting")
				}
				if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
					utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
				}
				goto done
			}
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Received input (len=%d)", w.Size())
			}
			var errorWhileWriting error = nil
			if errorWhileWriting := afw.__write__(item, w); errorWhileWriting != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileWriting, internals.SupressLogs)
			}
			// ---- Flush on failure
			if errorWhileWriting != nil {
				if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
					utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
				}
				goto done
			}
		case <-ticker.C:
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Ticker fired, flushing %d inputs", w.Size())
			}
			if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
				goto done
			}
		}
	}
done:
	return
}

/**
* Primary assumption behind @__drain__ is that no other routines are writing to afw.InputChannel
* Will bubble up errors and @Run() will write to afw.ErrorChannel
 */
func (afw *AsyncFileWriter[T]) __drain__(
	w *bufio.Writer,
) error {
	if len(afw.InputChannel) != 0 {
		for item := range afw.InputChannel {
			if err := afw.__write__(item, w); err != nil {
				return err
			}
			if len(afw.InputChannel) == 0 {
				break
			}
		}
	}
	return nil
}

func (afw *AsyncFileWriter[T]) __write__(
	input T,
	w *bufio.Writer,
) error {
	if _, err := w.Write(input.Serialize()); err != nil {
		return fmt.Errorf("Write error: %w", err)
	}
	return nil
}

func (afw *AsyncFileWriter[T]) __flush__(
	w *bufio.Writer,
) error {
	if err := w.Flush(); err != nil {
		return fmt.Errorf("Failed to flush: %w", err)
	}
	if !afw.Config.SupressLogs {
		utils.LogWithContext(utils.Info{}, "Successfully flushed %d bytes", w.Buffered())
	}
	return nil
}

// ---- Old API
func TrySendError(__errChan__ chan<- error, __err__ error, doLog bool) {
	select {
	case __errChan__ <- __err__:
	default:
		if doLog {
			utils.LogWithContext(utils.Fatal{}, "%v", __err__)
		}
	}
}

func flushAndSync(w *bufio.Writer, f *os.File) error {
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushAndSync: flush failed: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("flushAndSync: sync failed: %w", err)
	}
	return nil
}

// ---- WARNING readAndFlushLinks's intended behavior is to empty links array after processing
func readAndFlushLinks(
	w *bufio.Writer,
	f *os.File,
) error {
	if !supressLogs {
		utils.LogWithContext(utils.Info{}, "%d links to flush", len(*links))
	}
	for _, l := range *links {
		if _, err := w.WriteString(l); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	*links = (*links)[:0] // empty the slice
	if ferr := flushAndSync(w, f); ferr != nil {
		return fmt.Errorf("writeToFile: flushAndSync error: %v", ferr)
	}
	return nil
}

func WriteToFile(
	wdg *sync.WaitGroup,
	inputChannel <-chan string,
	errChannel chan<- error,
	allWritersAreDoneCh <-chan struct{},
	ctx context.Context,
	flushSize int,
	flushInterval time.Duration,
	f *os.File, // change this to Writer io.Writer later
	supressLogs bool,
) {
	defer func() {
		wdg.Done()
	}()
	if flushSize <= 0 {
		flushSize = 100
	}
	//---- Check if the file is writeable
	perms, err := os.Stat(f.Name())
	if err != nil {
		__err__ := fmt.Errorf("error getting file perms for %s: %w", f.Name(), err)
		utils.TrySendError(errChannel, __err__, false)
		return
	}
	if perms.Mode().Perm()&(0200|0020|0002) == 0 {
		__err__ := fmt.Errorf("%s is not writeable: %w", f.Name(), err)
		utils.TrySendError(errChannel, __err__, false)
		return
	}
	w := bufio.NewWriter(f)
	ticker := time.NewTicker(flushInterval)
	links := make([]string, 0, flushSize)
	defer func() {
		_ = w.Flush()
		_ = f.Sync()
	}()

	//---- Goroutine exits on:
	//----  context cancellation
	//----  input channel (linkChannel) closes
	//----  during errors found
	for {
		select {
		case <-allWritersAreDoneCh:
			//---- Assumption here is that no one will be writing to the inputChannel
			//---- because all writers are done
			if len(inputChannel) != 0 {
				for item := range inputChannel {
					links = append(links, item)
					if len(inputChannel) == 0 {
						break
					}
				}
			}
			if err := readAndFlushLinks(&links, w, f, supressLogs); err != nil {
				utils.TrySendError(errChannel, err, false)
			}
			return
		case <-ctx.Done():
			if !supressLogs {
				utils.LogWithContext(utils.Info{}, "context cancellation occurred:%v", ctx.Err())
			}
			return
		case link, ok := <-inputChannel:
			if !ok {
				if !supressLogs {
					utils.LogWithContext(utils.Info{}, "link channel closed, exiting")
				}
				if err := readAndFlushLinks(&links, w, f, supressLogs); err != nil {
					utils.TrySendError(errChannel, err, false)
				}
				return
			}
			if !supressLogs {
				utils.LogWithContext(utils.Info{}, "received input: %q (len=%d)", link, len(links))
			}
			links = append(links, link)
			if len(links) >= flushSize {
				if !supressLogs {
					utils.LogWithContext(utils.Info{}, "input buffer full, flusing (%d) inputs", len(links))
				}
				if err := readAndFlushLinks(&links, w, f, supressLogs); err != nil {
					utils.TrySendError(errChannel, err, false)
					return
				}
			}
		case <-ticker.C:
			if !supressLogs {
				utils.LogWithContext(utils.Info{}, "ticker fired, flushing %d inputs", len(links))
			}
			if err := readAndFlushLinks(&links, w, f, supressLogs); err != nil {
				utils.TrySendError(errChannel, err, false)
				return
			}
		}
	}
}
