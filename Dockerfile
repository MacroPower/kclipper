FROM golang:1.23

COPY kclx /usr/local/bin/

ENTRYPOINT ["kclx"]
