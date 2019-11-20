package main

import (
	"flag"
	"fmt"
	"os"
	"net"

	"github.com/midbel/dissect"
)

func main() {
	listen := flag.Bool("l", false, "listen")
	flag.Parse()

	var err error
	if *listen {
		err = dissectFromConn()
	} else {
		err = dissectFromFiles()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func dissectFromConn() error {
	r, err := os.Open(flag.Arg(1))
	if err != nil {
		return err
	}
	defer r.Close()

	a, err := net.ResolveUDPAddr("udp", flag.Arg(0))
	if err != nil {
		return err
	}
	var c net.Conn
	if a.IP.IsMulticast() {
		c, err = net.ListenMulticastUDP("udp", nil, a)
	} else {
		c, err = net.ListenUDP("udp", a)
	}
	if err != nil {
		return err
	}
	defer c.Close()

	return dissect.Dissect(r, c)
}

func dissectFromFiles() error {
	r, err := os.Open(flag.Arg(0))
	if err != nil {
		return err
	}
	defer r.Close()

	var files []string
	for i := 1; i < flag.NArg(); i++ {
		files = append(files, flag.Arg(i))
	}
	return dissect.DissectFiles(r, files)
}
