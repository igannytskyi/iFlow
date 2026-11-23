# iFlow, built and installed in one place.
#
#   make            build bin/iflow
#   make test       vet and test everything
#   make install    build, put this build in the app, and replace the running
#                   service with it, carrying its runs on
#
# The app is ~/Applications/iFlow.app; it is signed with the local
# certificate iFlowLocal, without which macOS shows no window and asks for
# its permissions again.

APP      ?= $(HOME)/Applications/iFlow.app
IDENTITY ?= iFlowLocal

.PHONY: build test install app service

build:
	go build -o bin/ ./cmd/...

test:
	gofmt -l internal cmd | (! grep .)
	go vet ./...
	go test ./...

install: build app service

app: build
	@if [ -d "$(APP)" ]; then \
		cp bin/iflow "$(APP)/Contents/MacOS/iflow" && \
		codesign --force -s "$(IDENTITY)" --identifier org.iflow.desk "$(APP)" && \
		echo "the app runs this build"; \
	else echo "no app at $(APP): the window runs as bin/iflow desk"; fi

service: build
	bin/iflow service restart
