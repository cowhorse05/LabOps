FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/labops-server ./cmd/server

FROM alpine:3.20

RUN addgroup -S labops && adduser -S labops -G labops
WORKDIR /app
COPY --from=build /out/labops-server /app/labops-server
RUN mkdir -p /data && chown -R labops:labops /data /app
USER labops
EXPOSE 8080

ENV LABOPS_ADDR=:8080
ENV LABOPS_DB_PATH=/data/labops.db

HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8080/api/health

ENTRYPOINT ["/app/labops-server"]
