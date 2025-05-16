APP := tbl
BIN := $(HOME)/.local/bin/$(APP)

build:
	go build -o $(APP)

install: build
	ln -sf $(PWD)/$(APP) $(BIN)

run: install
	$(APP)
