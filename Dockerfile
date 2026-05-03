FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /dockdater .

FROM scratch
COPY --from=builder /dockdater /dockdater

ENTRYPOINT ["/dockdater"]
