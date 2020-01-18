FROM golang:1.13.6-alpine3.11

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 CDOS=linux GOARCH=amd64 go build

ENTRYPOINT [ "/app/optionspy" ]
