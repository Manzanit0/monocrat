FROM golang:1.20-alpine3.16 as build

ARG SERVICE_PATH

RUN apk add git

WORKDIR /workspace

COPY . ./

RUN go mod vendor

RUN go build -o app ${SERVICE_PATH}

FROM alpine:3.16

# This is a dependency; the app needs it.
RUN apk add git

COPY --from=build /workspace/app /app

CMD ["/app"]
