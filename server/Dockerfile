FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=build /server /usr/local/bin/server
USER 65532:65532
EXPOSE 8081
CMD ["server"]
