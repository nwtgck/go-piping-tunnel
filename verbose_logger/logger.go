package verbose_logger

import (
	"fmt"
)

type Logger struct {
	Level int
}

func New(level int) *Logger {
	return &Logger{Level: level}
}

func (l *Logger) Log(messages ...string) {
	var idx int
	last := len(messages) - 1
	if l.Level < last {
		idx = l.Level
	} else {
		idx = last
	}
	fmt.Println(messages[idx])
}
