package tests

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sracha4355/GoStash/src/routines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ----------------------
// TEST SUITE
// ----------------------
type routineTestSuite struct {
	suite.Suite
	// Fields not covered by the Setup function
	// payload    routines.SerializableString
	// nProducers int
	// nPayloads  int
	// name       string

	// Fields covered by the Setup function
	file         *os.File
	afw          *routines.AsyncFileWriter[routines.SerializableString]
	bytesWritten atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc

	producerWg   sync.WaitGroup
	fileWriterWg sync.WaitGroup

	inputChannel       chan routines.SerializableString
	errorChannel       chan error
	doneChannel        chan struct{}
	inputChannelClosed bool
	errorChannelClosed bool
	doneChannelClosed  bool
}

// ----------------------
// HELPER METHODS
// ----------------------

// Launch a number of producers sending `nPayloads` each
func (suite *routineTestSuite) launchProducers(nProducers, nPayloads int, payload routines.SerializableString) {
	for i := 0; i < nProducers; i++ {
		suite.producerWg.Add(1)
		go func() {
			defer suite.producerWg.Done()
			for j := 0; j < nPayloads; j++ {
				select {
				case <-suite.ctx.Done():
					return
				case suite.inputChannel <- payload:
					suite.bytesWritten.Add(int64(payload.Len()))
				}
			}
		}()
	}
}

// Assert that all bytes sent were flushed to file
func (suite *routineTestSuite) assertAllBytesWritten() {
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), suite.bytesWritten.Load(), int64(len(buf)))
}

// ----------------------
// SETUP / TEARDOWN
// ----------------------
func (suite *routineTestSuite) SetupTest() {
	// Temp file
	dir := suite.T().TempDir()
	filePath := filepath.Join(dir, "write-here.txt")
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(suite.T(), err)
	suite.file = f

	// Channels
	IBUF, EBUF := 5, 5
	suite.inputChannel = make(chan routines.SerializableString, IBUF)
	suite.errorChannel = make(chan error, EBUF)
	suite.doneChannel = make(chan struct{})
	suite.inputChannelClosed = false
	suite.errorChannelClosed = false
	suite.doneChannelClosed = false

	// Context
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	// AsyncFileWriter
	suite.afw = &routines.AsyncFileWriter[routines.SerializableString]{
		Wg:           &suite.fileWriterWg,
		InputChannel: suite.inputChannel,
		ErrorChannel: suite.errorChannel,
		DoneChannel:  suite.doneChannel,
		Ctx:          suite.ctx,
		Config: routines.AsyncFileWriterConfig{
			WhenToFlush:   5,
			FlushInterval: 200 * time.Millisecond,
			SupressLogs:   true,
			Writer:        f,
		},
	}
}

func (suite *routineTestSuite) TearDownTest() {
	suite.cancel()
	suite.file.Close()
	if suite.inputChannelClosed == false {
		close(suite.inputChannel)
	}
	if suite.errorChannelClosed == false {
		close(suite.errorChannel)
	}
	if suite.doneChannelClosed == false {
		close(suite.doneChannel)
	}
	suite.bytesWritten.Store(0)
}

// ----------------------
// TESTS
// ----------------------
// func (suite *routineTestSuite) TestWriteToFileWithMultipleProducers() {
// 	PAYLOAD := routines.SerializableString{Value: "link\n"}
// 	GO_ROUTINES := 5
// 	NUMLINKS_TO_SEND_PER_PRODUCER := 5

// 	// Start writer
// 	suite.fileWriterWg.Add(1)
// 	go suite.afw.Run()

// 	// Start producers
// 	suite.launchProducers(GO_ROUTINES, NUMLINKS_TO_SEND_PER_PRODUCER, PAYLOAD)

// 	// Wait for producers
// 	suite.producerWg.Wait()

// 	// Close input channel so writer can finish
// 	close(suite.doneChannel)
// 	suite.doneChannelClosed = true

// 	// Wait for writer
// 	suite.fileWriterWg.Wait()

// 	// Assert
// 	suite.assertAllBytesWritten()
// }

func (suite *routineTestSuite) TestsAsyncFileWriterStressTest() {
	PAYLOAD := routines.SerializableString{Value: "stress\n"}
	PRODUCERS := 1000
	PAYLOADS_PER_PRODUCER := 1000

	suite.afw.Config.WhenToFlush = 10000
	suite.afw.Config.FlushInterval = 60 * time.Second
	suite.afw.Config.SupressLogs = true

	suite.fileWriterWg.Add(1)
	go suite.afw.Run()

	suite.launchProducers(PRODUCERS, PAYLOADS_PER_PRODUCER, PAYLOAD)
	suite.producerWg.Wait()

	close(suite.doneChannel)
	suite.doneChannelClosed = true

	suite.fileWriterWg.Wait()
	suite.assertAllBytesWritten()
}

func (suite *routineTestSuite) TestSendExactlyFlushSizeItems() {
	PAYLOAD := routines.SerializableString{Value: "x"}
	suite.afw.Config.WhenToFlush = 10
	suite.afw.Config.FlushInterval = 1000 * time.Hour

	suite.fileWriterWg.Add(1)
	go suite.afw.Run()

	for i := 0; i < suite.afw.Config.WhenToFlush; i++ {
		suite.inputChannel <- PAYLOAD
		suite.bytesWritten.Add(int64(PAYLOAD.Len()))
	}

	close(suite.doneChannel)
	suite.doneChannelClosed = true

	suite.fileWriterWg.Wait()
	suite.assertAllBytesWritten()
}

func (suite *routineTestSuite) TestLessThanFlushSizeUsesTicker() {
	PAYLOAD := routines.SerializableString{Value: "tick"}

	suite.afw.Config.WhenToFlush = 100
	suite.afw.Config.FlushInterval = 30 * time.Millisecond

	suite.fileWriterWg.Add(1)
	go suite.afw.Run()

	for i := 0; i < 3; i++ {
		suite.inputChannel <- PAYLOAD
		suite.bytesWritten.Add(int64(PAYLOAD.Len()))
	}

	time.Sleep(2 * suite.afw.Config.FlushInterval)

	close(suite.doneChannel)
	suite.doneChannelClosed = true

	suite.fileWriterWg.Wait()
	suite.assertAllBytesWritten()
}

func (suite *routineTestSuite) TestRemainingItemsFlushedOnEarlyClose() {
	PAYLOAD := routines.SerializableString{Value: "drain\n"}

	suite.fileWriterWg.Add(1)
	go suite.afw.Run()

	for i := 0; i < 1000; i++ {
		suite.inputChannel <- PAYLOAD
		suite.bytesWritten.Add(int64(PAYLOAD.Len()))
	}

	close(suite.inputChannel)
	suite.inputChannelClosed = true

	suite.fileWriterWg.Wait()
	suite.assertAllBytesWritten()
}

func (suite *routineTestSuite) TestDoneClosedWithEmptyInputChannel() {
	suite.fileWriterWg.Add(1)
	go suite.afw.Run()

	// No writes, close done immediately
	close(suite.doneChannel)
	suite.doneChannelClosed = true

	done := make(chan struct{})
	go func() {
		suite.fileWriterWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		suite.T().Fatal("writer stuck with empty input channel and done closed")
	}
}

func (suite *routineTestSuite) TestFullBufferedChannel() {
	PAYLOAD := routines.SerializableString{Value: "buf"}

	suite.afw.Config.WhenToFlush = 2
	suite.afw.Config.FlushInterval = 100 * time.Millisecond

	suite.fileWriterWg.Add(1)
	go suite.afw.Run()

	// Fill channel buffer
	for i := 0; i < cap(suite.inputChannel); i++ {
		suite.inputChannel <- PAYLOAD
		suite.bytesWritten.Add(int64(PAYLOAD.Len()))
	}

	close(suite.doneChannel)
	suite.doneChannelClosed = true

	suite.fileWriterWg.Wait()
	suite.assertAllBytesWritten()
}

func TestRoutineTestSuite(t *testing.T) {
	suite.Run(t, new(routineTestSuite))
}
