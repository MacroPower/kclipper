FROM golang:1.23

COPY kcl /usr/local/bin/

ENTRYPOINT ["kcl"]
