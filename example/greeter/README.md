# Greeter: Example Vice service

## Running the Greeter

1. Install NSQ [http://nsq.io/deployment/installing.html](http://nsq.io/deployment/installing.html)
1. In a shell, navigate to `/tmp` and run NSQ with the `nsqd` command
1. In another shell, navigate to `/greeter/service` and execute it with `go run main.go`
1. In another shell, navigate to `/greeter/client` and execute it with `go run main.go`
1. Type in names into the client, and see the responses coming from the service
