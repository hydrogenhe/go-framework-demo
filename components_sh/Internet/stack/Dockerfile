FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/go-framework-demo .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/frontend-server ./cmd/frontend-server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/go-framework-demo /app/go-framework-demo
COPY --from=build /out/frontend-server /app/frontend-server
COPY frontend /app/frontend
RUN mkdir -p /app/logs
EXPOSE 8080
ENTRYPOINT ["/app/go-framework-demo"]
