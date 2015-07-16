package main

import (
	"fmt"
)

var Verbose bool

func Error(format string, args ...interface{}) {
	log("ERROR: "+format, args...)
}

func Log(format string, args ...interface{}) {
	log(format, args...)
}

func LogV(format string, args ...interface{}) {
	if Verbose {
		log(format, args...)
	}
}

func log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)

}
