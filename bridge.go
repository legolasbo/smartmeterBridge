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
	SerialPort  string       `yaml:"serial_port"`
	DSMRVersion string       `yaml:"dsmr_version"`
	Server      ServerConfig `yaml:"server"`
	Verbose     bool         `yaml:"verbose"`
}

var config = Config{
	DSMRVersion: "4",
	Server: ServerConfig{
		Host: "",
		Port: 9988,
	},
	Verbose: false,
}

func (c Config) validate() {
	if c.SerialPort == "" {
		log.Fatal("Serial port is not configured")
	}
	c.Server.validate()
}

func getSerialConfig(port, dsmrVersion string) serial.Config {
	confs := map[string]serial.Config{
		"2": {
			Baud:     9600,
			Size:     7,
			Parity:   serial.ParityEven,
			StopBits: 1,
		},
		"4": {
			Baud:     115200,
			Size:     8,
			Parity:   serial.ParityNone,
			StopBits: 0,
		},
	}

	confs["2.2"] = confs["2"]
	confs["3"] = confs["2"]
	confs["5"] = confs["4"]
	confs["5B"] = confs["5"]
	confs["5L"] = confs["5"]
	confs["5S"] = confs["5"]
	confs["Q3D"] = confs["3"]

	conf, ok := confs[dsmrVersion]
	if !ok {
		log.Fatal("Unknown dsmr_version. Use 2, 2.2, 3, 4, 5, 5B, 5L, 5S or Q3D")
	}
	conf.Name = port

	return conf
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

	go ReadRawTelegrams(getSerialConfig(config.SerialPort, config.DSMRVersion), rawTelegrams)

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

				if config.Verbose {
					log.Println(telegram)
				}
			}
		}
	}
}

func ReadRawTelegrams(serialConfig serial.Config, rawTelegrams chan string) {
	lineChan := make(chan string)
	go readLines(serialConfig, lineChan)
	go collectTelegrams(lineChan, rawTelegrams)
}

func readLines(serialConfig serial.Config, rChan chan string) {
	s, err := serial.OpenPort(&serialConfig)
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
