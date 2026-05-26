package routines

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/sracha4355/GoStash/src/contracts"
	"github.com/sracha4355/GoStash/src/utils"
)

// ---- ideas for things to add
// errgroup with backpressure
// resuable worker pool
// channel drainer
// Atomic combo + wait group combo
// FanIn Helper
// Lifecycle-Owned Goroutine Manager
// Close-Once Channel Wrapper
// Context-Aware Semaphore
// “Flush-on-Exit”
// Singleflight-Lite
// Restricted Channel where only certain goroutines can write

type SerializableString struct {
	Value string
}

func (s SerializableString) Serialize() []byte {
	return []byte(s.Value)
}

func (s SerializableString) Len() int {
	return len(s.Value)
}

// AsyncFileWriterConfig controls batching, flush timing, and logging for AsyncFileWriter.
//
// WhenToFlush is the number of serialized items buffered before an automatic flush;
// values <= 0 default to 100 in Run. FlushInterval triggers periodic flushes via a
// ticker. SupressLogs disables informational logs. Writer receives flushed bytes.
type AsyncFileWriterConfig struct {
	WhenToFlush   int
	FlushInterval time.Duration
	SupressLogs   bool
	Writer        io.Writer
	itemsWritten  int
}
// AsyncFileWriter batches values from InputChannel, serializes each item, and
// writes through a buffered io.Writer. Run flushes when WhenToFlush items have
// been written, when FlushInterval elapses, or during shutdown.
//
// Basic usage:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	input := make(chan SerializableString, 128)
//	errs := make(chan error, 1)
//	done := make(chan struct{})
//	var wg sync.WaitGroup
//	wg.Add(1)
//	w := NewAsyncFileWriter(100, time.Second, false, os.Stdout, input, errs, done, ctx, &wg)
//	go w.Run()
//	input <- SerializableString{Value: "line\n"}
//	close(input)
//	wg.Wait()
//
// High throughput: use a large buffered input channel and a higher WhenToFlush
// (for example 1000) so Run amortizes flush syscalls across many items.
//
// Shutdown: Run exits when DoneChannel is closed, the context is cancelled, or
// InputChannel is closed. For DoneChannel and context cancellation, Run drains
// any immediately available items from InputChannel, then flushes. Errors are
// reported on ErrorChannel when possible.
type AsyncFileWriter[T interface {
	contracts.Serializable
	contracts.Len
}] struct {
	Wg           *sync.WaitGroup
	InputChannel <-chan T
	ErrorChannel chan<- error
	DoneChannel  <-chan struct{}
	Ctx          context.Context
	Config       AsyncFileWriterConfig
}

// NewAsyncFileWriter constructs an AsyncFileWriter. The caller must call Run in
// a goroutine and increment Wg before starting (Wg.Done is deferred in Run).
func NewAsyncFileWriter[T interface {
	contracts.Serializable
	contracts.Len
}](
	__when_to_flush__ int,
	__flush_interval__ time.Duration,
	__supress_logs__ bool,
	__writer__ io.Writer,
	__input_channel__ <-chan T,
	__error_channel__ chan<- error,
	__done_channel__ <-chan struct{},
	__ctx__ context.Context,
	__waitgroup__ *sync.WaitGroup,
) *AsyncFileWriter[T] {
	return &AsyncFileWriter[T]{
		Wg:           __waitgroup__,
		InputChannel: __input_channel__,
		ErrorChannel: __error_channel__,
		DoneChannel:  __done_channel__,
		Ctx:          __ctx__,
		Config: AsyncFileWriterConfig{
			WhenToFlush:   __when_to_flush__,
			FlushInterval: __flush_interval__,
			SupressLogs:   __supress_logs__,
			Writer:        __writer__,
			itemsWritten:  0,
		},
	}
}

// Run processes InputChannel until shutdown. It flushes on batch size, ticker,
// input close, DoneChannel, or context cancellation.
func (afw *AsyncFileWriter[T]) Run() {
	defer afw.Wg.Done()

	var internals *AsyncFileWriterConfig = &afw.Config
	if internals.WhenToFlush <= 0 {
		internals.WhenToFlush = 100
	}

	// -- pass value for buz size later
	w := bufio.NewWriter(internals.Writer)
	ticker := time.NewTicker(internals.FlushInterval)
	defer func() {
		_ = w.Flush()
		ticker.Stop()
	}()

	//---- Errors from helpers will bubble up to Run()
	//---- TrySendError() will send it to the ErrorChannel in a best-effort fashion
	for {
		select {
		case <-afw.DoneChannel:
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "DoneChannel closed")
			}
			if errorWhileDraining := afw.__drain__(w); errorWhileDraining != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileDraining, internals.SupressLogs)
			}
			//---- In the event of a drainage failure, we will still flush anything
			//---- that already made it into bufio.Writer's internal buffer
			if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
			}
			goto done
		//---- Potential change from ctx to errgroup later
		case <-afw.Ctx.Done():
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Context cancellation occurred:%v", afw.Ctx.Err())
			}
			if errorWhileDraining := afw.__drain__(w); errorWhileDraining != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileDraining, internals.SupressLogs)
			}
			//---- In the event of a drainage failure, we will still flush anything
			//---- that already made it into bufio.Writer's internal buffer
			if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
			}
			goto done
		case item, ok := <-afw.InputChannel:
			if !ok {
				if !internals.SupressLogs {
					utils.LogWithContext(utils.Info{}, "Input channel closed --- exiting")
				}
				//---- Can no longer read from InputChannel, so will flush and exit
				if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
					utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
				}
				goto done
			}
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Received input (len=%d)", internals.itemsWritten)
			}
			if errorWhileWriting := afw.__write__(item, w); errorWhileWriting != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileWriting, internals.SupressLogs)
				//---- Flush on failure
				if internals.itemsWritten >= internals.WhenToFlush {
					if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
						utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
					}
				}
				goto done
			}
		case <-ticker.C:
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Ticker fired, flushing %d inputs", internals.itemsWritten)
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
	itemsDrained := 0
	for {
		select {
		case item, ok := <-afw.InputChannel:
			if !ok {
				utils.LogWithContext(
					utils.Info{}, 
					"during __drain__ the input channel closed after draining %d items",
					 itemsDrained,
				)
				return nil
			}
			if err := afw.__write__(item, w); err != nil {
				return err
			}
			itemsDrained++
		default: // channel is empty
			return nil
		}
	}
}

func (afw *AsyncFileWriter[T]) __write__(
	input T,
	w *bufio.Writer,
) error {
	if _, err := w.Write(input.Serialize()); err != nil {
		utils.LogWithContext(utils.Info{}, "error while writing to buffer %v, flushing partially filled buffer", err)
		if flushErr := afw.__flush__(w); flushErr != nil {
			return fmt.Errorf("write error occurred, and an error in subsequent partial flush %w", flushErr)
		}
		return fmt.Errorf("write error occured %w", err)
	}
	afw.Config.itemsWritten++
	return nil
}

func (afw *AsyncFileWriter[T]) __flush__(
	w *bufio.Writer,
) error {
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}
	afw.Config.itemsWritten = 0
	if !afw.Config.SupressLogs {
		utils.LogWithContext(utils.Info{}, "Successfully flushed")
	}
	return nil
}
