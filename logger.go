package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"path"
	"runtime"
	"time"
)

//configure log format
func setLogger(){

	log.SetReportCaller(true)
	//log.SetFormatter(&log.JSONFormatter{})
	PopcornLogFormat := &log.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return fmt.Sprintf("%s:%d", filename, f.Line), time.Now().Format(time.RFC3339)
		},
	}
	log.SetFormatter(PopcornLogFormat)

	log.SetLevel(log.Level(LogLevel))
}