FROM golang:1.22

COPY kclx /usr/local/bin/

ENTRYPOINT ["kclx"]
