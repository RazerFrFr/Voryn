package utils

import (
	"fmt"
	"time"
)

func getTimestamp() string {
	now := time.Now()
	return now.Format("01/02/2006 15:04:05")
}

type LoggerFunc func(v ...any)

func createLogger(prefix string) LoggerFunc {
	return func(v ...any) {
		fmt.Print("[", getTimestamp(), "] ")
		fmt.Print("[", prefix, "] ")
		fmt.Println(v...)
	}
}

var Logger = struct {
	Backend LoggerFunc
	MongoDB LoggerFunc
	Warning LoggerFunc
	Error   LoggerFunc
	Debug   LoggerFunc
}{
	Backend: createLogger("BACKEND"),
	MongoDB: createLogger("MONGODB"),
	Warning: createLogger("WARNING"),
	Error:   createLogger("ERROR"),
	Debug:   createLogger("DEBUG"),
}
