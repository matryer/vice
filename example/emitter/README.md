# Emitter: Example Vice service

## Running

1. In a shell, install Emitter [https://emitter.io/download/](https://emitter.io/download/)
   - `go install github.com/emitter-io/emitter@latest && emitter`
   - The first run of emitter will create an `emitter.conf` file and
     print a new license key and a new secret key to stdout.
2. In the same shell, save the new license in an environment variable:
   - `export EMITTER_LICENSE=...`
3. In the same shell, start `emitter` again:
   - `emitter |& tee emitter.log`
4. In a new shell, save the new secret key in an environment variable:
   - `export EMITTER_SECRET_KEY=...`
   then navigate to `example/emitter/server` and execute it with `go run main.go`
5. In another new shell, save the new secret key in an environment variable:
   - `export EMITTER_SECRET_KEY=...`
   then navigate to `example/emitter/client` and execute it with `go run main.go`
6. Type in names into the client, and see the responses coming from the service

Note that there is no reason why you can start up _multiple_ servers and
_multiple_ clients, but realize that a message from a client will only be
received (and responded to) by one of the running servers and likewise the
response will be randomly received by one of the clients.
