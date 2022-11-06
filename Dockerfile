FROM golang:1.19 as builder

WORKDIR /build
COPY Makefile go.mod go.sum *.go ./
RUN make build

FROM fluent/fluent-bit:2.0.3

COPY --from=builder /build/dist/out_logexporter.so /fluent-bit/bin/
COPY fluent-bit.conf /etc/fluent-bit/fluent-bit.conf
COPY plugins.conf /etc/fluent-bit/plugins.conf

CMD ["--config", "/etc/fluent-bit/fluent-bit.conf"]
