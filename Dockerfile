FROM golang:1.24 AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/ppiankov/pgpulse/internal/cli.version=${VERSION}" -o /pgpulse ./cmd/pgpulse

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /pgpulse /pgpulse
EXPOSE 9187
ENTRYPOINT ["/pgpulse"]
CMD ["serve"]
