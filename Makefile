.DEFAULT_GOAL := build

build:clean
	mkdir ./bin
	go build -o ./bin/stfg main.go

run:
	go run main.go

clean:
	rm -rf ./bin
