# 做出 Go API 的镜像。镜像是打好的文件系统加启动命令，还没在跑。
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /out/server .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=build /out/server /server
USER nobody
EXPOSE 8080
ENTRYPOINT ["/server"]
