package io_progress

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
	"strings"
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
}

func NewIOProgress(reader io.Reader, writer io.Writer, messageWriter io.Writer, makeMessage func(progress *IOProgress) string) *IOProgress {
	return &IOProgress{reader: reader, writer: writer, messageWriter: messageWriter, StartTime: time.Now(), makeMessage: makeMessage}
}

func (progress *IOProgress) Read(p []byte) (int, error) {
	var n, err = progress.reader.Read(p)
	if err != nil {
		return n, err
	}
	progress.CurrReadBytes += uint64(n)
	go progress.displayIfShould()
	return n, nil
}

func (progress *IOProgress) Write(p []byte) (int, error) {
	n, err := progress.writer.Write(p)
	if err != nil {
		return n, err
	}
	progress.CurrWriteBytes += uint64(n)
	go progress.displayIfShould()
	return n, nil
}

func (progress *IOProgress) Close() error {
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

func (progress *IOProgress) displayIfShould() {
	if time.Since(progress.lastDisplayTime).Milliseconds() < 1000 {
		return
	}
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
