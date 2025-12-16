// Basic imports
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

// ---- Testing Helpers ---- //
func launchProducers(
	nProducers,
	nPayloads int,
	bytesWritten *atomic.Int64,
	payload routines.SerializableString,
	ctx context.Context,
	wg *sync.WaitGroup,
	inputChannel chan routines.SerializableString,
) {
	for i := 0; i < nProducers; i++ {
		wg.Add(1)
		go func() {
			n := nPayloads
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					goto done
				case inputChannel <- payload:
					bytesWritten.Add(int64(payload.Len()))
					n--
					if n == 0 {
						goto done
					}
				}
			}
		done:
		}()
	}
}
func assertAllBytesWritten(suite *routineTestSuite) {
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

//-------------------------------------------------------//

type routineTestSuite struct {
	suite.Suite
	afw  *routines.AsyncFileWriter[routines.SerializableString]
	file *os.File
	//---- concurrency needs
	producerWdg   sync.WaitGroup
	fileWriterWdg sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	bytesWritten  atomic.Int64

	InputChannelClosed bool
	ErrorChannelClosed bool
	DoneChannelClosed  bool
	InputChannel       chan routines.SerializableString
	ErrorChannel       chan error
	DoneChannel        chan struct{}
}

func (suite *routineTestSuite) SetupTest() {
	dir := suite.T().TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	filePath := filepath.Join(dir, "write-here.txt")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(suite.T(), err)

	suite.cancel = cancel
	suite.ctx = ctx
	suite.file = file
	suite.fileWriterWdg = sync.WaitGroup{}
	suite.producerWdg = sync.WaitGroup{}
	IBUF := 5
	EBUF := 5
	suite.InputChannel = make(chan routines.SerializableString, IBUF)
	suite.ErrorChannel = make(chan error, EBUF)
	suite.DoneChannel = make(chan struct{})
	suite.afw = &routines.AsyncFileWriter[routines.SerializableString]{
		Wg:           &suite.fileWriterWdg,
		InputChannel: suite.InputChannel,
		ErrorChannel: suite.ErrorChannel,
		DoneChannel:  suite.DoneChannel,
		Ctx:          ctx,
		Config: routines.AsyncFileWriterConfig{
			WhenToFlush:   5,
			FlushInterval: time.Millisecond * 200,
			SupressLogs:   false,
			Writer:        file,
		},
	}
	suite.InputChannelClosed = true
	suite.ErrorChannelClosed = true
	suite.DoneChannelClosed = true

}

// ---- clean up after each test
func (suite *routineTestSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
	}
	if suite.file != nil {
		suite.file.Close()
	}
	if !suite.DoneChannelClosed {
		close(suite.DoneChannel)
	}

	close(suite.InputChannel)
	close(suite.ErrorChannel)
	suite.bytesWritten.And(0)
}

func (suite *routineTestSuite) TestWriteToFileWithMultipleProducers() {
	//---- Arrange
	var producerWG sync.WaitGroup
	PAYLOAD := routines.SerializableString{Value: "link\n"}
	// fmt.Printf("%d",PAYLOAD.Len())
	suite.afw.Config.SupressLogs = false
	GO_ROUTINES := 5
	NUMLINKS_TO_SEND_PER_PRODUCER := 5
	launchProducers(
		GO_ROUTINES,
		NUMLINKS_TO_SEND_PER_PRODUCER,
		&suite.bytesWritten,
		PAYLOAD,
		suite.ctx,
		&suite.producerWdg,
		suite.InputChannel,
	)

	suite.fileWriterWdg.Add(1)
	go suite.afw.Run()
	//----  writers
	// go func() {
	// 	for __err__ := range suite.ErrorChannel {
	// 		log.Printf("error from ErrorChannel %s", __err__.Error())
	// 		suite.cancel()
	// 	}
	// }()
	producerWG.Wait()
	close(suite.DoneChannel)
	suite.fileWriterWdg.Wait()
	// assert
	assertAllBytesWritten(suite)
}
func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(routineTestSuite))
}

// func (suite *RoutineTestSuite) TestWriteToFileWithAWriterCancellationDuringOtherWrites() {
// 	//---- Arrange
// 	var producerWG sync.WaitGroup
// 	WRITER_GO_ROUTINES := 5
// 	NUMLINKS_TO_SEND_PER_PRODUCER := 5
// 	PAYLOAD := "link\n"

// 	//---- Act
// 	suite.wdg.Add(1)
// 	go routines.WriteToFile(
// 		&suite.wdg,
// 		suite.pipeline.LinkChannel(),
// 		suite.pipeline.ErrorChannel(),
// 		suite.ctx,
// 		suite.flushSize,
// 		suite.flushInterval,
// 		suite.file,
// 		suite.supressLogs,
// 	)

// 	for i := 0; i < WRITER_GO_ROUTINES; i++ {
// 		producerWG.Add(1)
// 		go writer(
// 			NUMLINKS_TO_SEND_PER_PRODUCER,
// 			&producerWG,
// 			suite.ctx,
// 			suite.pipeline.LinkChannel(),
// 			&suite.BYTES_WRITTEN,
// 			PAYLOAD,
// 		)
// 	}
// 	producerWG.Add(2)
// 	//---- goroutine to purposely block until a cancel occurs
// 	go func() {
// 		defer producerWG.Done()
// 		for range suite.ctx.Done() {
// 			return
// 		}
// 	}()
// 	go func() {
// 		defer producerWG.Done()
// 		time.Sleep(300 * time.Millisecond)
// 		suite.cancel()
// 	}()

// 	producerWG.Wait()
// 	suite.wdg.Wait()

// 	//---- Assert
// 	select {
// 	case <-suite.ctx.Done():
// 	default:
// 		require.FailNow(suite.T(), "context should have been canceled")
// 	}
// }

// func (suite *RoutineTestSuite) TestWriteToFileWhenLinkChannelGetsClosed() {
// 	//---- Arrange
// 	var producerWG sync.WaitGroup
// 	WRITER_GO_ROUTINES := 5
// 	NUMLINKS_TO_SEND_PER_PRODUCER := 5
// 	PAYLOAD := "link"

// 	//---- Act
// 	suite.wdg.Add(1)
// 	go routines.WriteToFile(
// 		&suite.wdg,
// 		suite.pipeline.LinkChannel(),
// 		suite.pipeline.ErrorChannel(),
// 		suite.ctx,
// 		suite.flushSize,
// 		suite.flushInterval,
// 		suite.file,
// 		suite.supressLogs,
// 	)

// 	for i := 0; i < WRITER_GO_ROUTINES; i++ {
// 		producerWG.Add(1)
// 		go writer(
// 			NUMLINKS_TO_SEND_PER_PRODUCER,
// 			&producerWG,
// 			suite.ctx,
// 			suite.pipeline.LinkChannel(),
// 			&suite.BYTES_WRITTEN,
// 			PAYLOAD,
// 		)
// 	}
// 	producerWG.Wait()
// 	producerWG.Add(1)
// 	//---- goroutine to purposely block until a cancel occurs
// 	go func() {
// 		defer producerWG.Done()
// 		suite.pipeline.Close()
// 	}()
// 	suite.wdg.Wait()
// 	//---- Assert
// 	suite.file.Seek(0, io.SeekStart)
// 	buf, err := io.ReadAll(suite.file)
// 	require.NoError(suite.T(), err)
// 	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))
// }

// func (suite *RoutineTestSuite) TestWriteToFileWhenInsufficentFilePerms() {
// 	dir := suite.T().TempDir()
// 	filePath := filepath.Join(dir, "no-write.txt")
// 	nonWritableFile, err := os.OpenFile(filePath, os.O_CREATE, 0o444) // read-only permissions
// 	require.NoError(suite.T(), err)

// 	suite.wdg.Add(1)
// 	go routines.WriteToFile(
// 		&suite.wdg,
// 		suite.pipeline.LinkChannel(),
// 		suite.pipeline.ErrorChannel(),
// 		suite.ctx,
// 		suite.flushSize,
// 		suite.flushInterval,
// 		nonWritableFile,
// 		suite.supressLogs,
// 	)
// 	suite.wdg.Wait()
// 	for __err__ := range suite.pipeline.ErrorChannel() {
// 		log.Println(__err__.Error())
// 		suite.pipeline.Close()
// 	}
// }

// func (suite *RoutineTestSuite) TestWriteToFileSendingLessThanFlushSizeItems() {
// 	//---- Arrange
// 	WRITER_GO_ROUTINES := 1
// 	NUMLINKS_TO_SEND_PER_PRODUCER := 1000
// 	suite.flushSize = NUMLINKS_TO_SEND_PER_PRODUCER + 1
// 	PAYLOAD := "link\n"
// 	suite.pipeline.Close()
// 	pipeline := webdriver.NewPipeline(1000, 3)
// 	suite.pipeline = &pipeline

// 	//---- Act
// 	suite.wdg.Add(1)
// 	go routines.WriteToFile(
// 		&suite.wdg,
// 		suite.pipeline.LinkChannel(),
// 		suite.pipeline.ErrorChannel(),
// 		suite.ctx,
// 		suite.flushSize,
// 		suite.flushInterval,
// 		suite.file,
// 		suite.supressLogs,
// 	)

// 	var producerWG sync.WaitGroup
// 	for i := 0; i < WRITER_GO_ROUTINES; i++ {
// 		producerWG.Add(1)
// 		go writer(
// 			NUMLINKS_TO_SEND_PER_PRODUCER,
// 			&producerWG,
// 			suite.ctx,
// 			suite.pipeline.LinkChannel(),
// 			&suite.BYTES_WRITTEN,
// 			PAYLOAD,
// 		)
// 	}

// 	producerWG.Wait()
// 	suite.pipeline.Close()
// 	suite.wdg.Wait()

// 	//---- Assert
// 	suite.file.Seek(0, io.SeekStart)
// 	buf, err := io.ReadAll(suite.file)
// 	require.NoError(suite.T(), err)
// 	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))
// }
// func (suite *RoutineTestSuite) TestWriteToFileSendingExactlyFlushSizeItems() {
// 	//---- Arrange
// 	WRITER_GO_ROUTINES := 5
// 	NUMLINKS_TO_SEND_PER_PRODUCER := 1000
// 	suite.flushSize = NUMLINKS_TO_SEND_PER_PRODUCER * WRITER_GO_ROUTINES
// 	suite.flushInterval = 30 * time.Second
// 	PAYLOAD := "link\n"

// 	//---- Act
// 	suite.wdg.Add(1)
// 	go routines.WriteToFile(
// 		&suite.wdg,
// 		suite.pipeline.LinkChannel(),
// 		suite.pipeline.ErrorChannel(),
// 		suite.ctx,
// 		suite.flushSize,
// 		suite.flushInterval,
// 		suite.file,
// 		suite.supressLogs,
// 	)

// 	var producerWG sync.WaitGroup
// 	for i := 0; i < WRITER_GO_ROUTINES; i++ {
// 		producerWG.Add(1)
// 		go writer(
// 			NUMLINKS_TO_SEND_PER_PRODUCER,
// 			&producerWG,
// 			suite.ctx,
// 			suite.pipeline.LinkChannel(),
// 			&suite.BYTES_WRITTEN,
// 			PAYLOAD,
// 		)
// 	}

// 	producerWG.Wait()
// 	suite.cancel()
// 	suite.wdg.Wait()

// 	//---- Assert
// 	suite.file.Seek(0, io.SeekStart)
// 	buf, err := io.ReadAll(suite.file)
// 	require.NoError(suite.T(), err)
// 	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))

// }

// func (suite *RoutineTestSuite) TestStressTestForWriteToFile() {
// 	//---- Arrange
// 	WRITER_GO_ROUTINES := 5
// 	NUMLINKS_TO_SEND_PER_PRODUCER := 2000
// 	PAYLOAD := "link\n"
// 	suite.pipeline.Close()
// 	pipeline := webdriver.NewPipeline(100, 3)
// 	suite.pipeline = &pipeline
// 	suite.flushSize = 200
// 	suite.supressLogs = true
// 	writingDone := make(chan struct{})

// 	//---- Act
// 	suite.wdg.Add(1)
// 	go routines.WriteToFile(
// 		&suite.wdg,
// 		suite.pipeline.LinkChannel(),
// 		suite.pipeline.ErrorChannel(),
// 		writingDone,
// 		suite.ctx,
// 		suite.flushSize,
// 		suite.flushInterval,
// 		suite.file,
// 		suite.supressLogs,
// 	)

// 	var producerWG sync.WaitGroup
// 	for i := 0; i < WRITER_GO_ROUTINES; i++ {
// 		producerWG.Add(1)
// 		go writer(
// 			NUMLINKS_TO_SEND_PER_PRODUCER,
// 			&producerWG,
// 			suite.ctx,
// 			suite.pipeline.LinkChannel(),
// 			&suite.BYTES_WRITTEN,
// 			PAYLOAD,
// 		)
// 	}

// 	producerWG.Wait()
// 	close(writingDone)
// 	suite.wdg.Wait()

// 	//---- Assert
// 	suite.file.Seek(0, io.SeekStart)
// 	buf, err := io.ReadAll(suite.file)
// 	require.NoError(suite.T(), err)
// 	assert.Equal(suite.T(), int(suite.BYTES_WRITTEN.Load()), len(buf))
// 	log.Printf("Test done")
// }
// func TestRoutingTestSuite(t *testing.T) {
// 	suite.Run(t, new(RoutineTestSuite))
// }
