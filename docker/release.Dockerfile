FROM node:20-alpine as frontend

WORKDIR /app
COPY external/ui/ /app

RUN npm install
RUN npm run build

FROM golang:1.21-alpine as backend

WORKDIR /app
COPY . /app

RUN go build -o /foreverbull cmd/server/main.go

FROM golang:1.21-alpine

WORKDIR /app
COPY --from=frontend /app/dist /app/ui
COPY --from=backend /foreverbull /foreverbull

ENV UI_STATIC_PATH=/app/ui

EXPOSE 8080

CMD [ "/foreverbull" ]
