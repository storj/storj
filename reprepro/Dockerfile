FROM nginx:stable
MAINTAINER Storj <storj@storj.io>


RUN apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -yq --no-install-recommends \
  reprepro dpkg-sig
RUN adduser --system --disabled-password --disabled-login \
          --home /srv/packages \
          --group reprepro
COPY default.conf /etc/nginx/conf.d/default.conf
RUN mkdir -p /var/packages/debian/conf
COPY distributions /var/packages/debian/conf/
COPY options /var/packages/debian/conf/
VOLUME /packages


