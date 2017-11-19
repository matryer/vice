#Testing using docker

If RabbitMQ is not installed on your machine, the easiest way to install it is using docker container.

## Installing and running RabbitMQ without management tools plugin for the first time

docker run -d --hostname rabbitmq-test-host --name rabbitmq-test -p 5672:5672 -p 15672:15672 rabbitmq:3

## Installing and running RabbitMQ with management tools plugin for the first time

docker run -d --hostname rabbitmq-test-host --name rabbitmq-test -p 5672:5672 -p 15672:15672 rabbitmq:3-management

## Starting container

docker start rabbitmq-test
