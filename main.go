package main

import (
	"log"
	"os"

	"github.com/marianogappa/screpdb/cmd"
	"github.com/marianogappa/screpdb/internal/crashreport"
)

func main() {
	crashreport.SetOpenBrowser(false)
	defer crashreport.Recover(false)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := cmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
