all: install

pkgs = ./sia-ant ./sia-antfarm

# fmt calls go fmt on all packages.
fmt:
	gofmt -s -l -w $(pkgs)

vet:
	go vet $(pkgs)

# install builds and installs binaries.
install:
	go install -race $(pkgs)

test:
	go test -race -v $(pkgs)
