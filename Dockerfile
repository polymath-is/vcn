# vcn - vChain CodeNotary
# 
# Copyright (c) 2018-2019 vChain, Inc. All Rights Reserved.
# This software is released under GPL3.
# The full license information can be found under:
# https://www.gnu.org/licenses/gpl-3.0.en.html

FROM golang:1.12-stretch as builder
WORKDIR /src
COPY . .
RUN GOOS=linux GOARCH=amd64 make static

FROM scratch
COPY --from=builder /src/vcn /bin/vcn
COPY --from=alpine /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT [ "/bin/vcn" ]