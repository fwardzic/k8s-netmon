FROM golang:1.23.1
WORKDIR /app
ENV https_proxy=http://proxy.esl.cisco.com
ENV http_proxy=http://proxy.esl.cisco.com
ENV no_proxy=10.3.0.1,10.3.0.10,10.3.0.0/16,10.2.0.0/16,172.28.184.0/24,10.30.120.0/24,172.20.71.0/24
COPY go.mod go.sum ./
COPY main.go ./
RUN go mod download
# COPY ./vendor /app/
RUN CGO_ENABLED=0 GOOS=linux go build -o /k8s-netmon 
CMD ["/k8s-netmon"]