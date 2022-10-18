package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/tarm/serial"

	"github.com/roaldnefs/go-dsmr"
)

const (
	ServerHost = ""
	ServerPort = "9988"
	ServerType = "tcp"
)

var AllowSerialPortFailure = false

func main() {
	telegrams := make(chan dsmr.Telegram)

	go ReadTelegrams("/dev/ttyUSB0", telegrams)

	go startServer()

	for t := range telegrams {
		log.Println(t)
	}
}

func startServer() {
	fmt.Println("Server Running...")
	server, err := net.Listen(ServerType, ServerHost+":"+ServerPort)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer server.Close()
	fmt.Println("Listening on " + ServerHost + ":" + ServerPort)
	fmt.Println("Waiting for client...")
	for {
		connection, err := server.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		fmt.Println("client connected")
		go processClient(connection)
	}
}

func processClient(connection net.Conn) {
	buffer := make([]byte, 1024)
	mLen, err := connection.Read(buffer)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
	}
	fmt.Println("Received: ", string(buffer[:mLen]))
	_, err = connection.Write([]byte("Thanks! Got your message:" + string(buffer[:mLen])))
	_ = connection.Close()
}

// ReadTelegrams reads telegrams from the given serial port into the given readout channel.
func ReadTelegrams(serialPort string, telegrams chan dsmr.Telegram) {
	lineChan := make(chan string)
	rawTelegrams := make(chan string)
	go readLines(serialPort, lineChan)
	go collectTelegrams(lineChan, rawTelegrams)
	parseTelegrams(rawTelegrams, telegrams)
}

func readLines(serialPort string, rChan chan string) {
	c := &serial.Config{
		Name: serialPort,
		Baud: 115200,
	}

	s, err := serial.OpenPort(c)
	if err != nil {
		if AllowSerialPortFailure {
			return
		}
		log.Fatal(err)
	}

	reader := bufio.NewReader(s)
	for {
		reply, err := reader.ReadString('\n')
		if err != nil {
			log.Println(err)
			continue
		}

		rChan <- reply
	}
}

func collectTelegrams(rChan chan string, tChan chan string) {
	telegram := ""
	foundStart := false

	for line := range rChan {
		if line[0] == '/' {
			foundStart = true
			telegram = ""
		}

		// We usually start halfway trough a telegram.
		// Which means that the first telegram would be corrupt.
		// We therefore ignore everything until the first telegram start.
		if !foundStart {
			continue
		}

		telegram += line

		// The last line of a telegram starts with an exclamation mark.
		if line[0] == '!' {
			tChan <- telegram
		}
	}
}

func parseTelegrams(rawTelegrams chan string, telegrams chan dsmr.Telegram) {
	for t := range rawTelegrams {
		telegram, err := dsmr.ParseTelegram(t)
		if err != nil {
			log.Println(err)
			continue
		}

		telegrams <- telegram
	}
}
