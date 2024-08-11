FROM golang:1.22-alpine as backend

WORKDIR /app
COPY . /app

RUN go build -o /foreverbull cmd/server/main.go

FROM golang:1.22-alpine

WORKDIR /app
COPY --from=backend /foreverbull /foreverbull

ENV UI_STATIC_PATH=/app/ui

EXPOSE 8080

CMD [ "/foreverbull" ]
