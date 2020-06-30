FROM registry.access.redhat.com/ubi8/ubi-minimal:8.2

# Update image
RUN microdnf update -y && rm -rf /var/cache/yum

ENV OPERATOR=/usr/local/bin/ham-deploy \
    USER_UID=1001 \
    USER_NAME=ham-deploy

# install operator binary
COPY build/_output/bin/ham-deploy ${OPERATOR}

COPY build/bin /usr/local/bin
COPY deploy/crds /usr/local/etc/hybridapp/crds
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
