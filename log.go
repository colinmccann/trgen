// set of functions for error handling
// modelled off https://github.com/microsoft/ethr
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type LogLevel int

const (
	LogLevelInfo LogLevel = iota
	LogLevelDebug
)

type logMessage struct {
	Time    string
	Level   string
	Message string
}

// we may want something like this, leaving it for now
// type logTestResults struct {
// 	Time                 string
// 	Type                 string
// 	RemoteAddr           string
// 	Protocol             string
// 	BitsPerSecond        string
// 	ConnectionsPerSecond string
// 	PacketsPerSecond     string
// 	AverageLatency       string
// }

// this is buffered perhaps to align with other output. Experiment with this...
var logChan = make(chan string, 64)
var doneChan = make(chan struct{})

func initLogging(logToFile bool, fileName string) {
	log.SetFlags(0)

	// TODO: once we shut down the channel properly, we should change this
	// so that it doesnt opening the logfile for stdout logging
	logFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("Unable to open the log file %s, Error: %v\n", fileName, err)
		return
	}

	log.SetOutput(os.Stdout)
	// if log to file
	if logToFile {
		log.SetOutput(logFile)
	}

	go runLogger(logFile)
}

func runLogger(logFile *os.File) {
	for {
		select {
		case s := <-logChan:
			log.Println(s)
		case <-doneChan:
			// TODO - this never gets called, unless we include a sleep at the end of the program (when terminateLogging is called). Fix me
			fmt.Println("Shutting down logger...")
			logFile.Close()
			close(logChan)
		}
	}

}

func terminateLogging() {
	doneChan <- struct{}{}
}

func logMsg(prefix, msg string) {
	logData := logMessage{}
	logData.Time = time.Now().UTC().Format(time.RFC3339)
	logData.Level = prefix
	logData.Message = msg
	logJSON, _ := json.Marshal(logData)
	logChan <- string(logJSON)
}

func logInfo(msg string) {
	logMsg("INFO", msg)
}

func logError(msg string) {
	logMsg("ERROR", msg)
}

func logDebug(msg string) {
	logMsg("DEBUG", msg)
}

// we may want something like this, leaving it for now
// func logResults(s []string) {
// 	logData := logTestResults{}
// 	logData.Time = time.Now().UTC().Format(time.RFC3339)
// 	logData.Type = "TestResult"
// 	logData.RemoteAddr = s[0]
// 	logData.Protocol = s[1]
// 	logData.BitsPerSecond = s[2]
// 	logData.ConnectionsPerSecond = s[3]
// 	logData.PacketsPerSecond = s[4]
// 	logData.AverageLatency = s[5]
// 	logJSON, _ := json.Marshal(logData)
// 	logChan <- string(logJSON)
// }
