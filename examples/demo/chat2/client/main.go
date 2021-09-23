package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	client "github.com/revzim/go-pomelo-client"
	"github.com/urfave/cli"
)

const (
	DefaultPomeloAddress = "127.0.0.1:8080/ws"
)

var (
	PomeloAddress string
	ServerToken   string
	ServerID      string
	PomeloClient  *client.Connector
)

func InitPomeloClient(addr string) {
	PomeloClient = client.NewConnector()

	err := PomeloClient.InitReqHandshake("0.6.0", "golang-websocket", nil, map[string]interface{}{"name": "dude"})
	if err != nil {
		log.Fatal("Pomelo Handshake err: ", err)
	}
	err = PomeloClient.InitHandshakeACK(1)
	if err != nil {
		panic(err)
	}
	// connected := false
	PomeloClient.Connected(func() {
		log.Printf("connected to server at: %s\n", addr)
		// connected = true
		err = PomeloClient.Request("room.join", nil, func(data []byte) {
			log.Println("room join:", string(data))
		})
		if err != nil {
			panic(err)
		}
	})
	go func() {
		err := PomeloClient.Run(addr, true, 30)
		if err != nil {
			panic(err)
		}
	}()
	PomeloClient.On("onNewUser", func(data []byte) {
		log.Println("onNewUser", string(data))
	})
	PomeloClient.On("onMembers", func(data []byte) {
		log.Println("onMembers", string(data))
	})

	PomeloClient.On("onMessage", func(data []byte) {
		log.Println("onMessage", string(data))
	})
	defer PomeloClient.Close()
}

func handleClose() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		PomeloClient.Close()
		os.Exit(1)
	}()
}

func main() {
	app := &cli.App{
		Name:  "test client to connect to amoeba server",
		Usage: "run the test client",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "ip",
				Value:       DefaultPomeloAddress,
				Usage:       "ip address of game server",
				Destination: &PomeloAddress,
			},
			&cli.StringFlag{
				Name:        "token",
				Value:       "",
				Usage:       "your jwt token",
				Destination: &ServerToken,
			},
			&cli.StringFlag{
				Name:        "id",
				Value:       "",
				Usage:       "your server id",
				Destination: &ServerID,
			},
		},
		Action: func(c *cli.Context) error {
			serverAddr := fmt.Sprintf("ws://%s?id=%s&token=%s", PomeloAddress, ServerID, ServerToken)
			log.Printf("attempting to connect to: %s...\n", serverAddr)
			InitPomeloClient(serverAddr)
			return nil // errors.New(fmt.Sprintf(`provided ip: %s | default: %v`, PomeloAddress, PomeloAddress == DefaultPomeloAddress))
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	handleClose()

	for {
		time.Sleep(time.Second)
	}
}
