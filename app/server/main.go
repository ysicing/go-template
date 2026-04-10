package main

import (
	"log"

	"github.com/ysicing/go-template/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}

