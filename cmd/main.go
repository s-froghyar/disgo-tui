package main

import (
	"github.com/s-froghyar/disgo-tui/configs"
	"github.com/s-froghyar/disgo-tui/internal/client"
	"github.com/s-froghyar/disgo-tui/internal/tui"
)

var (
	httpClient *client.DiscogsClient
)

func main() {
	c, err := configs.LoadConfig()
	if err != nil {
		panic(err)
	}
	httpClient = client.New()
	tui := tui.New(httpClient, c)
	if err = tui.Start(); err != nil {
		panic(err)
	}
}
