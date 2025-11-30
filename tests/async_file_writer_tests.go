// Basic imports
package tests

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sracha4355/GoStash/src/contracts"
	"github.com/sracha4355/GoStash/src/routines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// /*
// Test cases to write:
// - routine shutting down if a writer to the fileWriter closes channel
// DONE
// - cancel ctx and writer then shutdowns properly
// DONE
// - Stress Test
// - how writer handles when file perms don't work or file does not exist
// DONE
// - send exacty flushSize items
// - send less than flushSize to test the ticker code
// - remaining items are flushed after early channel closure
// - test behavior with a full buffered channel
// - New test case to write, the input channel is initally empty after the all writersDoneChannel is closed
//   need to test we do not get stuck in the for loop
// */

// ---- helper methods for testing
func launchProducers[T contracts.Serializable](
	nProducers,
	nPayloads int,
	bytesWritten *atomic.Int64,
	payload T,
	ctx context.Context,
	wg *sync.WaitGroup,
	inputChannel chan T,
) {
	for i := 0; i < nProducers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					goto done
				case inputChannel <- payload:
					bytesWritten.Add(int64(payload.Len()))
					nPayloads--
					if nPayloads == 0 {
						goto done
					}
				}
			}
		done:
			return
		}()
	}
}
func assertAllBytesWritten(suite RoutineTestSuite) {
	//---- Assert
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(
		suite.T(),
		suite.bytesWritten.Load(),
		int64(len(buf)),
	)
}

/**
TODO:
- unittests for writer goroutine
*/

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type RoutineTestSuite struct {
	suite.Suite

	FileWriterConfig routines.AsyncFileWriterConfig
	file             *os.File

	//---- concurrency needs
	wdg          sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	InputChannel chan string
	ErrorChannel chan error
	DoneChannel  chan struct{}

	supressLogs  bool
	bytesWritten atomic.Int64
}

func (suite *RoutineTestSuite) SetupTest() {
	dir := suite.T().TempDir()
	filePath := filepath.Join(dir, "write-here.txt")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(suite.T(), err)
	suite.FileWriterConfig = routines.AsyncFileWriterConfig{
		WhenToFlush:   5,
		FlushInterval: time.Millisecond * 200,
		SupressLogs:   false,
		Writer:        file,
	}
	suite.file = file
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.wdg = sync.WaitGroup{}
	suite.supressLogs = false
	IBUF := 5
	EBUF := 5
	suite.InputChannel = make(chan string, IBUF)
	suite.ErrorChannel = make(chan error, EBUF)
	suite.DoneChannel = make(chan struct{})
}

// ---- clean up after each test
func (suite *RoutineTestSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
	}
	if suite.file != nil {
		suite.file.Close()
	}
	close(suite.DoneChannel)
	close(suite.InputChannel)
	close(suite.ErrorChannel)
	suite.bytesWritten.And(0)
}

func (suite *RoutineTestSuite) TestWriteToFileWithMultipleProducers() {
	//---- Arrange
	var producerWG sync.WaitGroup
	PAYLOAD := "link\n"
	supressLogs := suite.supressLogs
	GO_ROUTINES := 5
	NUMLINKS_TO_SEND_PER_PRODUCER := 5

	//---- Assert
	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		suite.file,
		supressLogs,
	)
	//----  writers
	go func() {
		for __err__ := range suite.pipeline.ErrorChannel() {
			log.Printf("error from ErrorChannel %s", __err__.Error())
			suite.cancel()
		}
	}()

	for i := 0; i < GO_ROUTINES; i++ {
		producerWG.Add(1)
		go writer(
			NUMLINKS_TO_SEND_PER_PRODUCER,
			&producerWG,
			suite.ctx,
			suite.pipeline.LinkChannel(),
			&suite.BYTES_WRITTEN,
			PAYLOAD,
		)
	}

	producerWG.Wait()
	suite.pipeline.Close()
	suite.wdg.Wait()

	//---- Assert
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(
		suite.T(),
		suite.BYTES_WRITTEN.Load(),
		int64(len(buf)),
	)
}

func (suite *RoutineTestSuite) TestWriteToFileWithAWriterCancellationDuringOtherWrites() {
	//---- Arrange
	var producerWG sync.WaitGroup
	WRITER_GO_ROUTINES := 5
	NUMLINKS_TO_SEND_PER_PRODUCER := 5
	PAYLOAD := "link\n"

	//---- Act
	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		suite.file,
		suite.supressLogs,
	)

	for i := 0; i < WRITER_GO_ROUTINES; i++ {
		producerWG.Add(1)
		go writer(
			NUMLINKS_TO_SEND_PER_PRODUCER,
			&producerWG,
			suite.ctx,
			suite.pipeline.LinkChannel(),
			&suite.BYTES_WRITTEN,
			PAYLOAD,
		)
	}
	producerWG.Add(2)
	//---- goroutine to purposely block until a cancel occurs
	go func() {
		defer producerWG.Done()
		for range suite.ctx.Done() {
			return
		}
	}()
	go func() {
		defer producerWG.Done()
		time.Sleep(300 * time.Millisecond)
		suite.cancel()
	}()

	producerWG.Wait()
	suite.wdg.Wait()

	//---- Assert
	select {
	case <-suite.ctx.Done():
	default:
		require.FailNow(suite.T(), "context should have been canceled")
	}
}

func (suite *RoutineTestSuite) TestWriteToFileWhenLinkChannelGetsClosed() {
	//---- Arrange
	var producerWG sync.WaitGroup
	WRITER_GO_ROUTINES := 5
	NUMLINKS_TO_SEND_PER_PRODUCER := 5
	PAYLOAD := "link"

	//---- Act
	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		suite.file,
		suite.supressLogs,
	)

	for i := 0; i < WRITER_GO_ROUTINES; i++ {
		producerWG.Add(1)
		go writer(
			NUMLINKS_TO_SEND_PER_PRODUCER,
			&producerWG,
			suite.ctx,
			suite.pipeline.LinkChannel(),
			&suite.BYTES_WRITTEN,
			PAYLOAD,
		)
	}
	producerWG.Wait()
	producerWG.Add(1)
	//---- goroutine to purposely block until a cancel occurs
	go func() {
		defer producerWG.Done()
		suite.pipeline.Close()
	}()
	suite.wdg.Wait()
	//---- Assert
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))
}

func (suite *RoutineTestSuite) TestWriteToFileWhenInsufficentFilePerms() {
	dir := suite.T().TempDir()
	filePath := filepath.Join(dir, "no-write.txt")
	nonWritableFile, err := os.OpenFile(filePath, os.O_CREATE, 0o444) // read-only permissions
	require.NoError(suite.T(), err)

	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		nonWritableFile,
		suite.supressLogs,
	)
	suite.wdg.Wait()
	for __err__ := range suite.pipeline.ErrorChannel() {
		log.Println(__err__.Error())
		suite.pipeline.Close()
	}
}

func (suite *RoutineTestSuite) TestWriteToFileSendingLessThanFlushSizeItems() {
	//---- Arrange
	WRITER_GO_ROUTINES := 1
	NUMLINKS_TO_SEND_PER_PRODUCER := 1000
	suite.flushSize = NUMLINKS_TO_SEND_PER_PRODUCER + 1
	PAYLOAD := "link\n"
	suite.pipeline.Close()
	pipeline := webdriver.NewPipeline(1000, 3)
	suite.pipeline = &pipeline

	//---- Act
	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		suite.file,
		suite.supressLogs,
	)

	var producerWG sync.WaitGroup
	for i := 0; i < WRITER_GO_ROUTINES; i++ {
		producerWG.Add(1)
		go writer(
			NUMLINKS_TO_SEND_PER_PRODUCER,
			&producerWG,
			suite.ctx,
			suite.pipeline.LinkChannel(),
			&suite.BYTES_WRITTEN,
			PAYLOAD,
		)
	}

	producerWG.Wait()
	suite.pipeline.Close()
	suite.wdg.Wait()

	//---- Assert
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))
}
func (suite *RoutineTestSuite) TestWriteToFileSendingExactlyFlushSizeItems() {
	//---- Arrange
	WRITER_GO_ROUTINES := 5
	NUMLINKS_TO_SEND_PER_PRODUCER := 1000
	suite.flushSize = NUMLINKS_TO_SEND_PER_PRODUCER * WRITER_GO_ROUTINES
	suite.flushInterval = 30 * time.Second
	PAYLOAD := "link\n"

	//---- Act
	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		suite.file,
		suite.supressLogs,
	)

	var producerWG sync.WaitGroup
	for i := 0; i < WRITER_GO_ROUTINES; i++ {
		producerWG.Add(1)
		go writer(
			NUMLINKS_TO_SEND_PER_PRODUCER,
			&producerWG,
			suite.ctx,
			suite.pipeline.LinkChannel(),
			&suite.BYTES_WRITTEN,
			PAYLOAD,
		)
	}

	producerWG.Wait()
	suite.cancel()
	suite.wdg.Wait()

	//---- Assert
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))

}

func (suite *RoutineTestSuite) TestStressTestForWriteToFile() {
	//---- Arrange
	WRITER_GO_ROUTINES := 5
	NUMLINKS_TO_SEND_PER_PRODUCER := 2000
	PAYLOAD := "link\n"
	suite.pipeline.Close()
	pipeline := webdriver.NewPipeline(100, 3)
	suite.pipeline = &pipeline
	suite.flushSize = 200
	suite.supressLogs = true
	writingDone := make(chan struct{})

	//---- Act
	suite.wdg.Add(1)
	go routines.WriteToFile(
		&suite.wdg,
		suite.pipeline.LinkChannel(),
		suite.pipeline.ErrorChannel(),
		writingDone,
		suite.ctx,
		suite.flushSize,
		suite.flushInterval,
		suite.file,
		suite.supressLogs,
	)

	var producerWG sync.WaitGroup
	for i := 0; i < WRITER_GO_ROUTINES; i++ {
		producerWG.Add(1)
		go writer(
			NUMLINKS_TO_SEND_PER_PRODUCER,
			&producerWG,
			suite.ctx,
			suite.pipeline.LinkChannel(),
			&suite.BYTES_WRITTEN,
			PAYLOAD,
		)
	}

	producerWG.Wait()
	close(writingDone)
	suite.wdg.Wait()

	//---- Assert
	suite.file.Seek(0, io.SeekStart)
	buf, err := io.ReadAll(suite.file)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))
	log.Printf("Test done")
}
func TestRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(RoutineTestSuite))
}
