#!/bin/bash
echo "Killing redis server just in case..."
ps aux | grep -i 'redis-server' | awk {'print $2'} | xargs kill -9
# echo "Removing old database"
rm ./testdata/dump.rdb
echo "Starting redis server..."
redis-server ./testdata/redis-testconfig.conf &> /dev/null &
sleep 2
echo "Finally testing:"
go test -v | grep --color .
echo "Killing redis again..."
ps aux | grep -i 'redis-server' | awk {'print $2'} | xargs kill -9