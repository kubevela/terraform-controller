build:
	GOOS=linux GOARCH=amd64 go build -o log-collector .

image-push: build
	docker build -t loheagn/log-collector:1.0.0 .
	docker push loheagn/log-collector:1.0.0
