FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
# go.sum se ustvari ob prvem buildu
RUN go mod tidy && CGO_ENABLED=0 go build -o /out/server .

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
