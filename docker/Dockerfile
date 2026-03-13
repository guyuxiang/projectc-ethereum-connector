# Building stage
FROM docker.m.daocloud.io/golang:1.24

RUN apk add --update make

WORKDIR /go/src/github.com/guyuxiang/projectc-ethereum-connector

COPY . /go/src/github.com/guyuxiang/projectc-ethereum-connector

ENV CGO_ENABLED 0
ENV GOOS linux
ENV GO111MODULE=on
ENV TIMEZONE "Asia/Shanghai"
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

RUN make src.build
RUN mv build/pkg/cmd/projectc-ethereum-connector/projectc-ethereum-connector /go/bin

# Production stage
FROM docker.m.daocloud.io/golang:1.24
WORKDIR /go/bin

LABEL maintainer="guyuxiang <guyuxiang@qq.com>"

# copy the go binaries from the building stage
COPY --from=0 /go/bin /go/bin

COPY etc/projectc-ethereum-connector.yaml /etc/projectc-ethereum-connector/projectc-ethereum-connector.yaml

EXPOSE 80

ENTRYPOINT ["./projectc-ethereum-connector"]
