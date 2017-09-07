package main

import (
	"context"
	"log"
	"time"

	"github.com/matryer/vice/queues/redis"
	redisV5Client "gopkg.in/redis.v5"
)

// To run this, install NSQ and start it with nsqd command.
// Run this program: go run main.go
// Send a name: curl -d 'Mat' 'http://127.0.0.1:4151/pub?topic=names'

func NewRedisClient(option *redis.Options) {
	option.Client = redisV5Client.NewClient(
		&redisV5Client.Options{
			Network:  "tcp",
			Addr:     "127.0.0.1:6379",
			Password: "",
			DB:       0,
		})
	_, err := option.Client.Ping().Result()
	if err != nil {
		panic("redis client connects server failed. " + err.Error())
	}
	return
}

// Greeter is a service that greets people.
func Greeter(ctx context.Context, names <-chan []byte,
	greetings chan<- []byte, errs <-chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Println("finished")
			return
		case err := <-errs:
			log.Println("an error occurred:", err)
		case name := <-names:
			greeting := "Hello " + string(name)
			greetings <- []byte(greeting)
		}
	}
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	transport := redis.New(NewRedisClient)
	defer func() {
		transport.Stop()
		<-transport.Done()
	}()
	names := transport.Receive("names")
	greetings := transport.Send("greetings")
	Greeter(ctx, names, greetings, transport.ErrChan())
}
