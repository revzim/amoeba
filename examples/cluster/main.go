package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pingcap/errors"
	amoeba "github.com/revzim/amoeba"
	"github.com/revzim/amoeba/examples/cluster/chat"
	"github.com/revzim/amoeba/examples/cluster/gate"
	"github.com/revzim/amoeba/examples/cluster/master"
	"github.com/revzim/amoeba/serialize/json"
	"github.com/revzim/amoeba/session"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "amoebaClusterDemo"
	app.Author = "Lonng"
	app.Email = "heng@lonng.org"
	app.Description = "amoeba cluster demo"
	app.Commands = []cli.Command{
		{
			Name: "master",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "listen,l",
					Usage: "Master service listen address",
					Value: "127.0.0.1:34567",
				},
			},
			Action: runMaster,
		},
		{
			Name: "gate",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "master",
					Usage: "master server address",
					Value: "127.0.0.1:34567",
				},
				cli.StringFlag{
					Name:  "listen,l",
					Usage: "Gate service listen address",
					Value: "",
				},
				cli.StringFlag{
					Name:  "gate-address",
					Usage: "Client connect address",
					Value: "",
				},
			},
			Action: runGate,
		},
		{
			Name: "chat",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "master",
					Usage: "master server address",
					Value: "127.0.0.1:34567",
				},
				cli.StringFlag{
					Name:  "listen,l",
					Usage: "Chat service listen address",
					Value: "",
				},
			},
			Action: runChat,
		},
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Startup server error %+v", err)
	}
}

func srcPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func runMaster(args *cli.Context) error {
	listen := args.String("listen")
	if listen == "" {
		return errors.Errorf("master listen address cannot empty")
	}

	webDir := filepath.Join(srcPath(), "master", "web")
	log.Println("amoeba master server web content directory", webDir)
	log.Println("amoeba master listen address", listen)
	log.Println("Open http://127.0.0.1:12345/web/ in browser")

	http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir(webDir))))
	go func() {
		if err := http.ListenAndServe(":12345", nil); err != nil {
			panic(err)
		}
	}()

	// Register session closed callback
	session.Lifetime.OnClosed(master.OnSessionClosed)

	// Startup amoeba server with the specified listen address
	amoeba.Listen(listen,
		amoeba.WithMaster(),
		amoeba.WithComponents(master.Services),
		amoeba.WithSerializer(json.NewSerializer()),
		amoeba.WithDebugMode(),
	)

	return nil
}

func runGate(args *cli.Context) error {
	listen := args.String("listen")
	if listen == "" {
		return errors.Errorf("gate listen address cannot empty")
	}

	masterAddr := args.String("master")
	if masterAddr == "" {
		return errors.Errorf("master address cannot empty")
	}

	gateAddr := args.String("gate-address")
	if gateAddr == "" {
		return errors.Errorf("gate address cannot empty")
	}

	log.Println("Current server listen address", listen)
	log.Println("Current gate server address", gateAddr)
	log.Println("Remote master server address", masterAddr)

	// Startup amoeba server with the specified listen address
	amoeba.Listen(listen,
		amoeba.WithAdvertiseAddr(masterAddr),
		amoeba.WithClientAddr(gateAddr),
		amoeba.WithComponents(gate.Services),
		amoeba.WithSerializer(json.NewSerializer()),
		amoeba.WithIsWebsocket(true),
		amoeba.WithWSPath("/amoeba"),
		amoeba.WithCheckOriginFunc(func(_ *http.Request) bool { return true }),
		amoeba.WithDebugMode(),
	)
	return nil
}

func runChat(args *cli.Context) error {
	listen := args.String("listen")
	if listen == "" {
		return errors.Errorf("chat listen address cannot empty")
	}

	masterAddr := args.String("master")
	if listen == "" {
		return errors.Errorf("master address cannot empty")
	}

	log.Println("Current chat server listen address", listen)
	log.Println("Remote master server address", masterAddr)

	// Register session closed callback
	session.Lifetime.OnClosed(chat.OnSessionClosed)

	// Startup amoeba server with the specified listen address
	amoeba.Listen(listen,
		amoeba.WithAdvertiseAddr(masterAddr),
		amoeba.WithComponents(chat.Services),
		amoeba.WithSerializer(json.NewSerializer()),
		amoeba.WithDebugMode(),
	)

	return nil
}
