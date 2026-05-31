FROM golang:1.25.1-alpine

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN apk update && apk add --no-cache git && go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app .

ENV GIN_MODE=release

CMD ["app"]
