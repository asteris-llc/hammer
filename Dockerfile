FROM centos:7

MAINTAINER Brian Hicks <brian@aster.is>

# ruby
RUN yum install -y ruby ruby-devel make gcc \
 && gem install fpm \
 && yum remove -y ruby-devel make gcc

COPY . /go/src/github.com/asteris-llc/hammer
RUN yum install -y go git mercurial \
 && cd /go/src/github.com/asteris-llc/hammer \
 && export GOPATH=/go \
 && go get \
 && go build -o /bin/hammer \
 && rm -rf /go \
 && yum remove -y go git mercurial

ENTRYPOINT ["/bin/hammer"]
