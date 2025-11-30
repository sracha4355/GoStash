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

type SerializableString struct {
	Value string
}

func (s SerializableString) Serialize() []byte {
	return []byte(s.Value)
}

// ---- New API in progress ---- will need to refactor tests
type AsyncFileWriterConfig struct {
	WhenToFlush   int
	FlushInterval time.Duration
	SupressLogs   bool
	Writer        io.Writer
}
type AsyncFileWriter[T contracts.Serializable] struct {
	wg           *sync.WaitGroup
	InputChannel <-chan T
	ErrorChannel chan<- error
	DoneChannel  <-chan struct{}
	Ctx          context.Context
	Config       AsyncFileWriterConfig
}

func NewAsyncFileWriterr[T contracts.Serializable](
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
		wg:           __waitgroup__,
		InputChannel: __input_channel__,
		ErrorChannel: __error_channel__,
		DoneChannel:  __done_channel__,
		Ctx:          __ctx__,
		Config: AsyncFileWriterConfig{
			WhenToFlush:   __when_to_flush__,
			FlushInterval: __flush_interval__,
			SupressLogs:   __supress_logs__,
			Writer:        __writer__,
		},
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
	w := bufio.NewWriter(internals.Writer)
	ticker := time.NewTicker(internals.FlushInterval)
	defer func() {
		_ = w.Flush()
	}()

	//---- Errors from helpers will bubble up to Run()
	//---- TrySendError() will send it to the ErrorChannel in a best-effort fashion
	for {
		select {
		case <-afw.DoneChannel:
			if errorWhileDraining := afw.__drain__(w); errorWhileDraining != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileDraining, internals.SupressLogs)
			}
			//---- In the event of a drainage failure, we will still flush anything
			//---- that already made it into bufio.Writer's internal buffer
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
				//---- Can no longer read from InputChannel, so will flush and exit
				if errorWhileFlushing := afw.__flush__(w); errorWhileFlushing != nil {
					utils.TrySendError(afw.ErrorChannel, errorWhileFlushing, internals.SupressLogs)
				}
				goto done
			}
			if !internals.SupressLogs {
				utils.LogWithContext(utils.Info{}, "Received input (len=%d)", w.Size())
			}
			if errorWhileWriting := afw.__write__(item, w); errorWhileWriting != nil {
				utils.TrySendError(afw.ErrorChannel, errorWhileWriting, internals.SupressLogs)
				//---- Flush on failure
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
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}

func (afw *AsyncFileWriter[T]) __flush__(
	w *bufio.Writer,
) error {
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}
	if !afw.Config.SupressLogs {
		utils.LogWithContext(utils.Info{}, "Successfully flushed")
	}
	return nil
}
