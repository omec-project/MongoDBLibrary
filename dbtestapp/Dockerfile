# Copyright 2019-present Open Networking Foundation
#
# SPDX-License-Identifier: Apache-2.0
#

FROM golang:1.14.4-stretch AS test

LABEL maintainer="ONF <omec-dev@opennetworking.org>"

RUN apt-get update
RUN apt-get -y install vim 
RUN cd $GOPATH/src && mkdir -p dbtestapp
COPY . $GOPATH/src/dbtestapp
RUN cd $GOPATH/src/dbtestapp && go install 

FROM test AS dbtestapp
WORKDIR /dbtestapp
RUN mkdir -p /dbtestapp/bin
COPY --from=test $GOPATH/bin/* /dbtestapp/bin/
WORKDIR /dbtestapp
