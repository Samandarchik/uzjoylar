# Stage 1: Go build 
FROM golang:1.23 AS builder 
 
WORKDIR /app 
 
COPY go.mod go.sum ./ 
RUN go mod download 
 
COPY . . 
 
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags migrate -o amur ./
 
# Stage 2: Prepare runtime files in alpine 
FROM alpine:latest AS alpine-stage 
 
WORKDIR /app 
 
RUN apk add --no-cache ca-certificates tzdata 
 
COPY --from=builder /app/amur /app/amur
COPY --from=builder /app/data /app/data 
 
ENV TZ=Asia/Tashkent 
RUN ln -snf /usr/share/zoneinfo/Asia/Tashkent /etc/localtime && echo "Asia/Tashkent" > /etc/timezone 
 
RUN chmod +x /app/amur 
 
EXPOSE 2020
 
CMD ["/app/amur"]