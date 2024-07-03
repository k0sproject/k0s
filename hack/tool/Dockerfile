FROM debian:bullseye AS builder
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip openssh-client

FROM golang:1.19-bullseye AS builder-tool
COPY . /go/src/tool
WORKDIR /go/src/tool
RUN go build -o /tool

FROM builder AS builder-terraform
ARG ARCH
ENV TERRAFORM_VERSION=1.3.3
RUN curl --proto '=https' --tlsv1.2 --retry 5 --retry-all-errors -Lfo /terraform.zip https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${ARCH}.zip && unzip /terraform.zip

FROM builder AS builder-aws
ARG HARDWARE
RUN curl --proto '=https' --tlsv1.2 --retry 5 --retry-all-errors -fo "/tmp/awscliv2.zip" "https://awscli.amazonaws.com/awscli-exe-linux-${HARDWARE}.zip" && \
    cd /tmp && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf /tmp/aws

FROM builder-aws AS final
RUN mkdir -p /tool/bin /tool/terraform/bin /tool/terraform/scripts /tool/data
COPY --from=builder-terraform /terraform /tool/terraform/bin
COPY --from=builder-tool /tool /tool/bin/
COPY terraform /tool/terraform/scripts/

ENV PATH=/tool/bin:/tool/terraform/bin:$PATH
ENTRYPOINT ["/tool/bin/tool"]
