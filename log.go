package main

import (
	"io"
	"log"
)

var (
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func setLogStreams(
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {
	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(infoHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(infoHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

}
