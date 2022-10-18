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
	//telegrams := make(chan dsmr.Telegram)
	rawTelegrams := make(chan string)

	//go ReadTelegrams("/dev/ttyUSB0", telegrams)
	go ReadRawTelegrams("/dev/ttyUSB0", rawTelegrams)

	newConnections := make(chan net.Conn)
	go startServer(newConnections)
	sendTelegrams(newConnections, rawTelegrams)
}

func startServer(clients chan net.Conn) {
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
		clients <- connection
		fmt.Println("client connected")
	}
}

func sendTelegrams(connections chan net.Conn, telegrams chan string) {
	clients := make([]net.Conn, 0)
	select {
	case connection := <-connections:
		clients = append(clients, connection)
	case telegram := <-telegrams:
		for i, c := range clients {
			_, err := c.Write([]byte(telegram))
			if err != nil {
				_ = c.Close()
				clients = append(clients[:i], clients[i+1:]...)
				log.Println("client disconnected")
				continue
			}
			log.Println(telegram)
		}
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
	rawTelegrams := make(chan string)
	ReadRawTelegrams(serialPort, rawTelegrams)
	parseTelegrams(rawTelegrams, telegrams)
}

func ReadRawTelegrams(serialPort string, rawTelegrams chan string) {
	lineChan := make(chan string)
	go readLines(serialPort, lineChan)
	go collectTelegrams(lineChan, rawTelegrams)
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
