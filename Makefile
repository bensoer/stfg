.DEFAULT_GOAL := build

LLAMA_DIR := res/llama-go
LIB_DIR := lib
LLAMA_STAMP := $(LLAMA_DIR)/.built

LLAMA_LIBS := $(wildcard $(LLAMA_DIR)/*.a)

build: $(LLAMA_STAMP) copy-libs
	CGO_LDFLAGS="-L$(LIB_DIR)" go build -o ./bin/stfg main.go

$(LLAMA_STAMP):
	mkdir -p res
	git clone --recurse-submodules https://github.com/tcpipuk/llama-go $(LLAMA_DIR)
	cd $(LLAMA_DIR) && make CPPFLAGS+=' -DLLAMA_DISABLE_LOGS=1' libbinding.a
	touch $@

copy-libs:
	mkdir -p $(LIB_DIR)
	cp $(LLAMA_DIR)/*.a $(LIB_DIR)/

run:
	CGO_LDFLAGS="-L$(LIB_DIR)" go run main.go

clean:
	rm -rf ./res
	rm -rf ./lib
	rm -rf ./bin