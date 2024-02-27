FROM golang:1.21 AS build
## based on debian 11

RUN useradd -u 10001 dimo

WORKDIR /go/src/github.com/dimo-network/trips-web-app/
COPY /api /go/src/github.com/dimo-network/trips-web-app/
COPY /app /go/src/github.com/dimo-network/trips-web-app/

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOFLAGS=-mod=vendor
ENV DIMO_API_BASEURL=/api/v1
ENV DIMO_IDENTITY_API=https://identity-api.dimo.zone
ENV DIMO_IDENTITY_DEV_API=https://identity-api.dev.dimo.zone

RUN apt-get clean && apt-get update
RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
RUN apt-get install -y nodejs
RUN go mod download
RUN go mod tidy
RUN go mod vendor
RUN make install
RUN npm install && npm run build

FROM busybox AS package

LABEL maintainer="DIMO <hello@dimo.zone>"

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/github.com/dimo-network/trips-web-app/target/bin/trips-web-app .
COPY --from=build /go/src/github.com/dimo-network/trips-web-app/dist /dist

USER dimo

EXPOSE 8080
#EXPOSE 8888

CMD /trips-web-app
