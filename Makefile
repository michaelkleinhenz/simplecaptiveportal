BINARY := captive-portal
SERVICE := captive-portal.service

.PHONY: build build-pi build-arm clean deps install install-remote run test tidy

# Build for current platform
build:
	go build -o $(BINARY) .

# Download module dependencies (optional; go build fetches automatically)
deps:
	go mod download

# Build for Raspberry Pi Zero W 2 (armv7)
build-pi:
	GOOS=linux GOARCH=arm GOARM=7 go build -o $(BINARY) .

# Build for generic Linux ARM (e.g. Pi 3/4)
build-arm:
	GOOS=linux GOARCH=arm go build -o $(BINARY) .

clean:
	rm -f $(BINARY)

# Install binary and systemd unit (run with sudo for system install).
# Service needs ports 80 (portal) and 53 (captive DNS).
install: build-pi
	install -Dm755 $(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)
	install -Dm644 $(SERVICE) $(DESTDIR)/etc/systemd/system/$(SERVICE)
	@if [ -z "$(DESTDIR)" ]; then \
		systemctl daemon-reload; \
		systemctl enable $(SERVICE); \
		systemctl restart $(SERVICE); \
		echo "Started/restarted $(SERVICE)"; \
	fi

# Copy binary and service to remote over SSH and install there. Requires REMOTE=user@host.
# Example: make install-remote REMOTE=pi@192.168.1.100
install-remote: build-pi
	@if [ -z "$(REMOTE)" ]; then \
		echo "Usage: make install-remote REMOTE=user@host"; \
		echo "Example: make install-remote REMOTE=pi@raspberrypi.local"; \
		exit 1; \
	fi
	scp $(BINARY) $(SERVICE) $(REMOTE):/tmp/
	ssh $(REMOTE) "sudo install -m755 /tmp/$(BINARY) /usr/local/bin/$(BINARY) && \
		sudo install -m644 /tmp/$(SERVICE) /etc/systemd/system/$(SERVICE) && \
		sudo systemctl daemon-reload && \
		sudo systemctl enable $(SERVICE) && \
		sudo systemctl restart $(SERVICE) && \
		rm -f /tmp/$(BINARY) /tmp/$(SERVICE) && \
		echo 'Installed and started $(SERVICE) on $(REMOTE)'"

run: build
	./$(BINARY)

test:
	go test ./...

tidy:
	go mod tidy
