FROM golang:alpine as builder

WORKDIR /go/src/xybSign

COPY . .

RUN go env -w GO111MODULE=on \ 
 && go env -w GOPROXY=https://goproxy.cn,direct \ 
 && go env -w CGO_ENABLED=0 \ 
 && go env \ 
 && go mod tidy \ 
 && go build -o mybinary . 

ENTRYPOINT ./mybinary