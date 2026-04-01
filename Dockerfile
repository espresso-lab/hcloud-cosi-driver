#
# BUILDER
#
FROM docker.io/library/golang:1.24 AS builder

WORKDIR /buildroot

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/

RUN CGO_ENABLED=0 go build -trimpath -o build/hcloud-cosi-driver cmd/cosi-driver/main.go

#
# FINAL IMAGE
#
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /buildroot/build/hcloud-cosi-driver /hcloud-cosi-driver

USER nonroot:nonroot

ENTRYPOINT [ "/hcloud-cosi-driver" ]
