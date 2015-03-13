package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

var port int
var baseWebDir = "."

func init() {
	fmt.Println("Simple HTTP Server")

	portFlag := flag.Int("port", 8085, "optional, port number; default: 8085")
	baseWebFlag := flag.String("web", ".", "optional, web dir; default: .")

	flag.Parse()
	port = int(*portFlag)
	baseWebDir = string(*baseWebFlag)
}

/*
This is a simple HTTP server which responds with HTML,
relative to the executed directory.
*/
func main() {

	fmt.Printf("Serving files in the %s directory on port %v...\n", baseWebDir, port)

	// Server
	http.Handle("/", http.FileServer(http.Dir(baseWebDir)))
	err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
