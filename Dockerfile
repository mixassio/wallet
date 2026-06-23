FROM golang:1.22

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o server ./cmd/server

EXPOSE 3000

CMD ["/app/server"]
