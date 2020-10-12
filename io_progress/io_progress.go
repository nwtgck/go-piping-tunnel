package io_progress

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type IOProgress struct {
	io.Reader
	CurrReadBytes   uint64
	reader          io.Reader
	CurrWriteBytes  uint64
	StartTime       time.Time
	messageWriter   io.Writer
	makeMessage     func(progress *IOProgress) string
	maxMessageLen   int
	lastDisplayTime time.Time
}

func NewIOProgress(reader io.Reader, messageWriter io.Writer, makeMessage func(progress *IOProgress) string) IOProgress {
	return IOProgress{reader: reader, messageWriter: messageWriter, StartTime: time.Now(), makeMessage: makeMessage}
}

func (progress *IOProgress) Read(p []byte) (int, error) {
	var n, err = progress.reader.Read(p)
	if err != nil {
		return n, err
	}
	progress.displayIfShould()
	progress.CurrReadBytes += uint64(n)
	return n, nil
}

func (progress *IOProgress) Write(p []byte) (int, error) {
	l := len(p)
	progress.CurrWriteBytes += uint64(l)
	progress.displayIfShould()
	return l, nil
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
