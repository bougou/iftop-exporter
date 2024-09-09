FROM ubuntu:22.04 as base
ENV DEBIAN_FRONTEND=noninteractive

RUN true && \
  apt-get update && \
  apt-get -y upgrade && \
  apt-get install -y gpg locales unzip && \
  locale-gen zh_CN.UTF-8 && \
  apt-get install -y iftop iproute2 telnet wget curl net-tools vim && \
  apt-get clean && rm -rf /var/lib/apt/lists/* && \
  true

ENV TERM xterm
ENV LANG zh_CN.UTF-8
ENV LANGUAGE zh_CN:en
ENV LC_ALL zh_CN.UTF-8


FROM golang:1.22 as builder
COPY / /src
RUN cd /src \
  && make build \
  && mv /src/_output/iftop-exporter /iftop-exporter \
  && rm -rf /src \
  && true


FROM base
WORKDIR /
COPY --from=builder /iftop-exporter .
RUN mkdir -p /var/lib/iftop-exporter/dynamic
ENTRYPOINT [ "/iftop-exporter" ]
