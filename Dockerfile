# build
FROM golang:1.26-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/feedsanctum ./cmd/server

# runtime
FROM alpine:3.10

WORKDIR /app
COPY --from=build /out/feedsanctum /app/feedsanctum
EXPOSE 8080
CMD [ "/app/feedsanctum" ]

