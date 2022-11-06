
build: dist/out_logexporter.so

dist/out_logexporter.so: go.mod go.sum logexporter.go
	go build -ldflags '-s -w' -buildmode=c-shared -o dist/out_logexporter.so .

test:
	go test

format:
	goimports -w .

lint:
	golangci-lint run

distclean:
	rm dist/out_logexporter.so

clean: distclean

.PHONY: build distclean clean lint format test
