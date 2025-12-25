package main

import (
	"log"
	"os"

	"github.com/marianogappa/screpdb/cmd"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := cmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
