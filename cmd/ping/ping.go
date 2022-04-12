package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/tatsushid/go-fastping"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Walterpepe/go-fastping"
)

type response struct {
	addr *net.IPAddr
	rtt  time.Duration
}

func main() {
	var filename string
	flag.StringVar(&filename, "f", "target.txt", "read list of targets from a file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s [options] hostname [source]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	fh, err := os.OpenFile(filename, os.O_RDWR, 0666)
	defer fh.Close()
	if err != nil {
		log.Printf("Open file %s error! %v", filename, err)
		return
	}

	var targets []string
	buf := bufio.NewReader(fh)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				//fmt.Println("File read ok!")
				break
			} else {
				log.Println("Read file error!", err)
				return
			}
		}
		if strings.TrimSpace(line) != "" {
			targets = append(targets, line)
		}
	}

	p := fastping.NewPinger()
	netProto := "ip4:icmp"

	// 填入探测地址
	results := make(map[string]*response)

	for _, target := range targets {
		ra, err := net.ResolveIPAddr(netProto, target)
		if err != nil {
			continue
		}
		results[ra.String()] = nil
		p.AddIPAddr(ra)
	}

	onRecv, onIdle := make(chan *response), make(chan bool)
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		onRecv <- &response{addr: addr, rtt: t}
	}
	p.OnIdle = func() {
		onIdle <- true
	}

	p.MaxRTT = time.Second
	p.RunLoop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

loop:
	for {
		select {
		case <-c:
			fmt.Println("get interrupted")
			break loop
		case res := <-onRecv:
			if _, ok := results[res.addr.String()]; ok {
				results[res.addr.String()] = res
			}
		case <-onIdle:
			for host, r := range results {
				if r == nil {
					fmt.Printf("%s : unreachable %v\n", host, time.Now())
				} else {
					fmt.Printf("%s : %v %v\n", host, r.rtt, time.Now())
				}
				results[host] = nil
			}
		case <-p.Done():
			if err = p.Err(); err != nil {
				fmt.Println("Ping failed:", err)
			}
			break loop
		}
	}
	signal.Stop(c)
	p.Stop()
}
