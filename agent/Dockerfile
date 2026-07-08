FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/labops-agent ./cmd/agent

FROM alpine:3.20

RUN addgroup -S labops && adduser -S labops -G labops
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/labops-agent /app/labops-agent
USER labops

ENTRYPOINT ["/app/labops-agent"]
