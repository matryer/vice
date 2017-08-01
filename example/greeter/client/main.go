package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/matryer/vice/queues/nsq"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()
	transport := nsq.New()
	names := transport.Send("names")
	greetings := transport.Receive("greetings")
	go func() {
		for greeting := range greetings {
			fmt.Println(string(greeting))
		}
	}()
	go func() {
		fmt.Println("Type some names to send through the |names| channel:")
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			b := s.Bytes()
			if len(b) == 0 {
				continue
			}
			names <- b
		}
		if err := s.Err(); err != nil {
			log.Println(err)
		}
	}()
	<-ctx.Done()
	transport.Stop()
	<-transport.Done()
	log.Println("transport stopped")
}
