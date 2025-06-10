package logger

import (
	"log"
)

var debugEnabled bool = false

func Init(debug bool) {
	debugEnabled = debug
}

func Debug(format string, v ...interface{}) {
	if debugEnabled {
		log.Printf("[DEBUG] "+format, v...)
	}
}
