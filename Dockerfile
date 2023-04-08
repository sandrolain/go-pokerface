FROM golang:1.20.3 as builder
WORKDIR /build
COPY ./src/ .
RUN CGO_ENABLED=0 go build -o ./pkrface ./pokerface

FROM golang:1.20.3-alpine3.17
WORKDIR /usr/src/app
COPY --from=builder /build/pkrface ./pkrface
CMD ["./pkrface", "/config.json"]
