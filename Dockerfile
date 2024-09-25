FROM golang:1.23.1
WORKDIR /app
COPY go.mod go.sum ./
COPY main.go ./
COPY ./vendor ./vendor
RUN go build -mod=vendor -o /k8s-netmon 
CMD ["/k8s-netmon"]