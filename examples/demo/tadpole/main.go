package main

import (
	"log"
	"net/http"
	"os"

	amoeba "github.com/revzim/amoeba"
	"github.com/revzim/amoeba/component"
	"github.com/revzim/amoeba/examples/demo/tadpole/logic"
	"github.com/revzim/amoeba/serialize/json"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()

	app.Name = "tadpole"
	app.Author = "amoeba authors"
	app.Version = "0.0.1"
	app.Copyright = "amoeba authors reserved"
	app.Usage = "tadpole"

	// flags
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "addr",
			Value: ":23456",
			Usage: "game server address",
		},
	}

	app.Action = serve

	app.Run(os.Args)
}

func serve(ctx *cli.Context) error {
	components := &component.Components{}
	components.Register(logic.NewManager())
	components.Register(logic.NewWorld())

	// register all service
	options := []amoeba.Option{
		amoeba.WithIsWebsocket(true),
		amoeba.WithComponents(components),
		amoeba.WithSerializer(json.NewSerializer()),
		amoeba.WithCheckOriginFunc(func(_ *http.Request) bool { return true }),
	}

	//amoeba.EnableDebug()
	log.SetFlags(log.LstdFlags | log.Llongfile)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := ctx.String("addr")
	amoeba.Listen(addr, options...)
	return nil
}
