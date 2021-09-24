package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	client "github.com/revzim/amoeba-client"
	"github.com/revzim/amoeba/crypt"
	"github.com/urfave/cli"
)

const (
	DefaultAmoebaAddress = "localhost:80/ws"
)

var (
	AmoebaAddress string
	ServerToken   string
	ServerID      string
	AmoebaClient  *client.Connector

	c = crypt.New([]byte(""))
)

func DecryptPacket(msg []byte) (map[string]interface{}, error) {
	decodedMsg := crypt.Decode(string(msg))
	// log.Println("decrypt", msg, string(decodedMsg))
	dataBytes, err := c.Decrypt(msg)
	if err != nil {
		return nil, err
	}
	var data interface{}
	err = json.Unmarshal(dataBytes, &data)
	if err != nil {
		log.Println("err parsing json bytes", string(msg), decodedMsg, string(dataBytes))
		return nil, err
	}
	// log.Println(data)
	return data.(map[string]interface{}), nil
}

func InitAmoebaClient(addr string) {
	AmoebaClient = client.NewConnector()

	err := AmoebaClient.InitReqHandshake("0.6.0", "golang-websocket", nil, map[string]interface{}{"name": "dude"})
	if err != nil {
		log.Fatal("Amoeba Handshake err: ", err)
	}
	err = AmoebaClient.InitHandshakeACK(1)
	if err != nil {
		panic(err)
	}
	// connected := false
	AmoebaClient.Connected(func() {
		log.Printf("connected to server at: %s\n", addr)
		// connected = true
		err = AmoebaClient.Request("room.join", nil, func(data []byte) {
			decPkt, _ := DecryptPacket(data)
			log.Println("onJoinRoom", decPkt)
		})
		if err != nil {
			panic(err)
		}
	})
	go func() {
		err := AmoebaClient.Run(addr, true, 10)
		if err != nil {
			panic(err)
		}
	}()
	AmoebaClient.On("onNewUser", func(data []byte) {
		decPkt, _ := DecryptPacket(data)
		log.Println("onNewUser", decPkt) // string(data))
	})
	AmoebaClient.On("onMembers", func(data []byte) {
		decPkt, _ := DecryptPacket(data)
		log.Println("onMembers", decPkt) // string(data))
	})

	AmoebaClient.On("onMessage", func(data []byte) {
		decPkt, _ := DecryptPacket(data)
		log.Println("onMessage", decPkt) // string(data))
	})
	defer AmoebaClient.Close()
}

func handleClose() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		AmoebaClient.Close()
		os.Exit(1)
	}()
}

func main() {
	app := &cli.App{
		Name:  "test client to connect to nano server",
		Usage: "run the test client",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "ip",
				Value:       DefaultAmoebaAddress,
				Usage:       "ip address of game server",
				Destination: &AmoebaAddress,
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
			serverAddr := fmt.Sprintf("ws://%s?id=%s&token=%s", AmoebaAddress, ServerID, ServerToken)
			log.Printf("attempting to connect to: %s...\n", serverAddr)
			InitAmoebaClient(serverAddr)
			return nil // errors.New(fmt.Sprintf(`provided ip: %s | default: %v`, AmoebaAddress, AmoebaAddress == DefaultAmoebaAddress))
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
