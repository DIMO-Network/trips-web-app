FROM golang:1.21 AS build
## based on debian 11

RUN useradd -u 10001 dimo

WORKDIR /go/src/github.com/dimo-network/trips-web-app/
COPY /api /go/src/github.com/dimo-network/trips-web-app/
COPY /app-login /go/src/github.com/dimo-network/trips-web-app/

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOFLAGS=-mod=vendor
ENV DIMO_API_BASEURL= 
ENV DIMO_PROJECT_ID=d16b9a51a03ecf7d3a617416144f70fb

RUN apt-get clean && apt-get update
RUN curl -fsSL https://deb.nodesource.com/setup_current.x | bash -
RUN apt-get install -y nodejs
RUN npm install -g typescript
RUN npm install -g vite
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
COPY --from=build /go/src/github.com/dimo-network/trips-web-app/views /views
COPY --from=build /go/src/github.com/dimo-network/trips-web-app/static /static

USER dimo

EXPOSE 8080
#EXPOSE 8888

CMD /trips-web-app
