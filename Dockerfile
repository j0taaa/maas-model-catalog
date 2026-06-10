FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /maas-model-catalog .

FROM scratch

ENV PORT=3000

COPY --from=builder /maas-model-catalog /maas-model-catalog

USER 65532:65532

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=3 CMD ["/maas-model-catalog", "-healthcheck"]

CMD ["/maas-model-catalog"]
