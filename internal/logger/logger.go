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

func Info(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}

func Warn(format string, v ...interface{}) {
	log.Printf("[WARN] "+format, v...)
}

func Error(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}
