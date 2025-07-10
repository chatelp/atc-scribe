package audio

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// MultiReader implements a reader that can be consumed by multiple goroutines
type MultiReader struct {
	buffer     []byte // Circular buffer for audio data
	bufferSize int    // Size of the circular buffer
	writeIndex int    // Current write position in the buffer
	readers    map[string]*readerState
	mu         sync.RWMutex // Mutex for thread safety
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *logger.Logger
	closed     bool
}

// readerState tracks the state of each reader
type readerState struct {
	readIndex int        // Current read position in the circular buffer
	readCond  *sync.Cond // Condition variable for signaling new data
	closed    bool
}

// NewMultiReader creates a new multi-reader
func NewMultiReader(ctx context.Context, logger *logger.Logger) *MultiReader {
	bufferSize := 1024 * 64 // 64KB buffer for low latency (about 1.3 seconds at 24kHz mono)
	readerCtx, readerCancel := context.WithCancel(ctx)

	mr := &MultiReader{
		buffer:     make([]byte, bufferSize),
		bufferSize: bufferSize,
		writeIndex: 0,
		readers:    make(map[string]*readerState),
		ctx:        readerCtx,
		cancel:     readerCancel,
		logger:     logger,
		closed:     false,
	}

	return mr
}

// Write writes data to the buffer and notifies all readers
func (mr *MultiReader) Write(p []byte) (n int, err error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if mr.closed {
		return 0, io.ErrClosedPipe
	}

	// Copy data to the circular buffer
	n = len(p)
	for i := 0; i < n; i++ {
		mr.buffer[mr.writeIndex] = p[i]
		mr.writeIndex = (mr.writeIndex + 1) % mr.bufferSize
	}

	// Notify all readers that new data is available
	for _, reader := range mr.readers {
		if !reader.closed && reader.readCond != nil {
			reader.readCond.Signal()
		}
	}

	return n, nil
}

// CreateReader creates a new reader for the multi-reader
func (mr *MultiReader) CreateReader(id string) io.ReadCloser {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Check if reader already exists
	if reader, exists := mr.readers[id]; exists {
		if !reader.closed {
			return newMultiReaderClient(mr, id)
		}
		// If it exists but is closed, remove it and create a new one
		delete(mr.readers, id)
	}

	// Create a new reader state
	readerMutex := &sync.Mutex{}
	reader := &readerState{
		readIndex: mr.writeIndex,             // Start reading from current write position
		readCond:  sync.NewCond(readerMutex), // Condition variable for signaling
		closed:    false,
	}

	mr.readers[id] = reader
	mr.logger.Debug("Created new reader", logger.String("reader_id", id))

	return newMultiReaderClient(mr, id)
}

// RemoveReader removes a reader
func (mr *MultiReader) RemoveReader(id string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if reader, exists := mr.readers[id]; exists {
		// Mark as closed
		reader.closed = true

		// Signal the reader in case it's waiting
		if reader.readCond != nil {
			reader.readCond.Signal()
		}

		// Remove from map
		delete(mr.readers, id)
		mr.logger.Debug("Removed reader", logger.String("reader_id", id))
	}
}

// Close closes the multi-reader and all readers
func (mr *MultiReader) Close() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if mr.closed {
		return nil
	}

	mr.closed = true
	mr.cancel()

	// Close all readers
	for id, reader := range mr.readers {
		// Mark as closed
		reader.closed = true

		// Signal the reader in case it's waiting
		if reader.readCond != nil {
			reader.readCond.Signal()
		}

		mr.logger.Debug("Closed reader during shutdown", logger.String("reader_id", id))
	}

	// Clear readers map
	mr.readers = make(map[string]*readerState)

	return nil
}

// multiReaderClient is a ReadCloser that reads from a MultiReader
type multiReaderClient struct {
	mr *MultiReader
	id string
	mu sync.Mutex // Mutex for thread safety
}

// newMultiReaderClient creates a new client for the multi-reader
func newMultiReaderClient(mr *MultiReader, id string) io.ReadCloser {
	return &multiReaderClient{
		mr: mr,
		id: id,
	}
}

// Read reads data from the multi-reader
func (mrc *multiReaderClient) Read(p []byte) (n int, err error) {
	// Lock to prevent concurrent reads from the same client
	mrc.mu.Lock()
	defer mrc.mu.Unlock()

	// Get reader state
	mrc.mr.mu.RLock()
	reader, exists := mrc.mr.readers[mrc.id]
	if !exists || reader.closed || mrc.mr.closed {
		mrc.mr.mu.RUnlock()
		return 0, io.EOF
	}

	// Get current read position and buffer size
	readIndex := reader.readIndex
	writeIndex := mrc.mr.writeIndex
	bufferSize := mrc.mr.bufferSize
	readCond := reader.readCond
	mrc.mr.mu.RUnlock()

	// If there's no data available, wait for it
	if readIndex == writeIndex {
		// Wait for data with a timeout
		waitChan := make(chan struct{})

		go func() {
			readCond.L.Lock()
			defer readCond.L.Unlock()

			// Wait for signal or timeout
			readCond.Wait()
			close(waitChan)
		}()

		// Wait for either data or context cancellation
		select {
		case <-waitChan:
			// Data is available, continue
		case <-mrc.mr.ctx.Done():
			return 0, io.EOF
		case <-time.After(30 * time.Second):
			// Longer timeout, and return EOF to signal connection should be reestablished
			return 0, io.EOF
		}

		// Re-check state after waiting
		mrc.mr.mu.RLock()
		reader, exists = mrc.mr.readers[mrc.id]
		if !exists || reader.closed || mrc.mr.closed {
			mrc.mr.mu.RUnlock()
			return 0, io.EOF
		}
		readIndex = reader.readIndex
		writeIndex = mrc.mr.writeIndex
		mrc.mr.mu.RUnlock()
	}

	// Calculate how much data is available
	var available int
	if writeIndex > readIndex {
		available = writeIndex - readIndex
	} else {
		available = bufferSize - readIndex + writeIndex
	}

	// Limit to buffer size
	if available > len(p) {
		available = len(p)
	}

	// Copy data from circular buffer to output buffer
	copied := 0
	for copied < available {
		// Calculate contiguous chunk size
		chunkSize := available - copied
		if readIndex+chunkSize > bufferSize {
			chunkSize = bufferSize - readIndex
		}

		// Lock for reading from buffer
		mrc.mr.mu.RLock()
		copy(p[copied:copied+chunkSize], mrc.mr.buffer[readIndex:readIndex+chunkSize])
		mrc.mr.mu.RUnlock()

		copied += chunkSize
		readIndex = (readIndex + chunkSize) % bufferSize
	}

	// Update read position
	mrc.mr.mu.Lock()
	if reader, exists := mrc.mr.readers[mrc.id]; exists && !reader.closed {
		reader.readIndex = readIndex
	}
	mrc.mr.mu.Unlock()

	return copied, nil
}

// Close closes the reader
func (mrc *multiReaderClient) Close() error {
	mrc.mr.RemoveReader(mrc.id)
	return nil
}
