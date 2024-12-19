FROM golang:1.23

ENV KCL_FAST_EVAL=1
COPY kcl /usr/local/bin/

ENTRYPOINT ["kcl"]
