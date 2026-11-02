FROM golang:1.26.10 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
