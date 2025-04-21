
build:
	mkdir -p tmp
	go build -o tmp/skills main.go

dev:
	air -c .air.toml

run:
	./tmp/skills

install:
	go build -o tmp/yafai main.go
	sudo cp tmp/yafai /usr/local/bin

proto-gen:
		protoc --go_out=proto --go-grpc_out=proto proto/skill.proto; 

proto-rm:
		rm -rf proto/*pb.go