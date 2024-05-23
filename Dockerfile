FROM golang:1.22.1

ENV TODO_PORT 7540
ENV TODO_DBFILE ./scheduler.db

WORKDIR /app_go

COPY . .

RUN go mod download

COPY *.go ./
COPY web ./web
COPY tests ./tests
COPY models ./models
COPY database ./database

EXPOSE 7540
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /my_app

CMD ["/my_app"]