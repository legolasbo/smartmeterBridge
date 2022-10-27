package main

import (
	"bufio"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/go-yaml/yaml"
	"github.com/tarm/serial"
)

const (
	ServerType = "tcp"
)

var Paths = []string{"/etc/smartmeter-bridge/config.yml", "config.yml"}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int64  `yaml:"port"`
}

func (c ServerConfig) validate() {
	if c.Port < 0 {
		log.Fatal("Server port number must be positive or 0 for automatic port selection")
	}
}

type Config struct {
	SerialPort string       `yaml:"serial_port"`
	Server     ServerConfig `yaml:"server"`
}

var config = Config{}

func (c Config) validate() {
	if c.SerialPort == "" {
		log.Fatal("Serial port is not configured")
	}
	c.Server.validate()
}

func main() {
	config.SerialPort = os.Getenv("SMB_SERIAL_PORT")
	config.Server.Host = os.Getenv("SMB_SERVER_HOST")
	config.Server.Port, _ = strconv.ParseInt(os.Getenv("SMB_SERVER_PORT"), 10, 64)
	for _, path := range Paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		err = yaml.Unmarshal(b, &config)
		if err != nil {
			log.Fatal(err)
		}
	}

	config.validate()

	rawTelegrams := make(chan string)

	go ReadRawTelegrams(config.SerialPort, rawTelegrams)

	newConnections := make(chan net.Conn)
	go startServer(newConnections)
	sendTelegrams(newConnections, rawTelegrams)
}

func startServer(clients chan net.Conn) {
	log.Println("Starting server...")
	server, err := net.Listen(ServerType, config.Server.Host+":"+strconv.FormatInt(config.Server.Port, 10))
	if err != nil {
		log.Println("Could not start server:", err.Error())
		os.Exit(1)
	}
	defer func(server net.Listener) {
		_ = server.Close()
	}(server)

	log.Println("Listening on", server.Addr().String())
	for {
		c, err := server.Accept()
		if err != nil {
			log.Println("Error accepting", c.RemoteAddr(), err.Error())
			_ = c.Close()
			continue
		}
		clients <- c
		log.Println(c.RemoteAddr(), "connected")
	}
}

func sendTelegrams(connections chan net.Conn, telegrams chan string) {
	clients := make([]net.Conn, 0)
	for {
		select {
		case connection := <-connections:
			clients = append(clients, connection)
		case telegram := <-telegrams:
			for i, c := range clients {
				_, err := c.Write([]byte(telegram))
				if err != nil {
					_ = c.Close()
					clients = append(clients[:i], clients[i+1:]...)
					log.Println(c.RemoteAddr(), "disconnected")
					continue
				}
				log.Println(telegram)
			}
		}
	}
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

		// We usually start halfway through a telegram.
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
