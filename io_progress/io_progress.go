package io_progress

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
	"strings"
	"sync"
	"time"
)

type IOProgress struct {
	CurrReadBytes   uint64
	reader          io.Reader
	writer          io.Writer
	CurrWriteBytes  uint64
	StartTime       time.Time
	messageWriter   io.Writer
	makeMessage     func(progress *IOProgress) string
	maxMessageLen   int
	lastDisplayTime time.Time
	finDisplayCh    chan struct{}
	isCloseCalled   bool
	// To process .Close() once
	closeMutex *sync.Mutex
}

func NewIOProgress(writer io.Writer, reader io.Reader, messageWriter io.Writer, makeMessage func(progress *IOProgress) string) *IOProgress {
	p := &IOProgress{
		reader:        reader,
		writer:        writer,
		messageWriter: messageWriter,
		StartTime:     time.Now(),
		makeMessage:   makeMessage,
		finDisplayCh:  make(chan struct{}, 1),
		closeMutex:    new(sync.Mutex),
		isCloseCalled: false,
	}
	go func() {
		// Loop for displaying progress
		for len(p.finDisplayCh) == 0 {
			p.displayProgress()
			time.Sleep(1 * time.Second)
		}
	}()
	return p
}

func (progress *IOProgress) Read(p []byte) (int, error) {
	var n, err = progress.reader.Read(p)
	if err != nil {
		return n, err
	}
	progress.CurrReadBytes += uint64(n)
	return n, nil
}

func (progress *IOProgress) Write(p []byte) (int, error) {
	n, err := progress.writer.Write(p)
	if err != nil {
		return n, err
	}
	progress.CurrWriteBytes += uint64(n)
	return n, nil
}

func (progress *IOProgress) Close() error {
	// Lock to process once
	progress.closeMutex.Lock()
	// Unlock after processing
	defer progress.closeMutex.Unlock()
	// If already closed
	if progress.isCloseCalled {
		return nil
	}
	// Notify finish to display loop
	progress.finDisplayCh <- struct{}{}
	progress.isCloseCalled = true

	var rErr error
	var wErr error
	if r, ok := progress.reader.(io.ReadCloser); ok {
		rErr = r.Close()
	}
	if w, ok := progress.writer.(io.WriteCloser); ok {
		wErr = w.Close()
	}
	return util.CombineErrors(wErr, rErr)
}

func (progress *IOProgress) displayProgress() {
	// Make message
	message := progress.makeMessage(progress)
	// Clear & show message
	spaces := strings.Repeat(" ", progress.maxMessageLen)
	fmt.Fprintf(progress.messageWriter, "\r"+spaces+"\r"+message)
	if len(message) > progress.maxMessageLen {
		progress.maxMessageLen = len(message)
	}
	progress.lastDisplayTime = time.Now()
}
