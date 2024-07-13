FROM golang:1.22-alpine

WORKDIR /app

COPY . /app

RUN echo "" > .env

RUN go build -o /foreverbull cmd/server/main.go

EXPOSE 8080

CMD [ "/foreverbull" ]
