ARG LDAP_VERSION=2.6.8
FROM bitnamilegacy/openldap:${LDAP_VERSION}

ARG LDAP_VERSION
USER root

# ビルドに必要な依存関係をインストール
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    gcc \
    libc6-dev \
    libldap2-dev \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# OpenLDAP ソースコードの取得と pw-bcrypt のコンパイル
RUN wget https://www.openldap.org/software/download/OpenLDAP/openldap-release/openldap-${LDAP_VERSION}.tgz -O /tmp/openldap.tgz \
    && tar -xzf /tmp/openldap.tgz -C /tmp \
    && rm /tmp/openldap.tgz \
    && cd /tmp/openldap-${LDAP_VERSION} \
    && ./configure --disable-slapd \
    && make depend \
    && cd libraries && make && cd .. \
    && git clone https://github.com/wclarie/openldap-bcrypt.git /tmp/openldap-${LDAP_VERSION}/contrib/slapd-modules/passwd/bcrypt \
    && cd /tmp/openldap-${LDAP_VERSION}/contrib/slapd-modules/passwd/bcrypt \
    && sed -i 's/libldap_r/libldap/g' Makefile \
    && make \
    && cp .libs/pw-bcrypt.so /opt/bitnami/openldap/lib/openldap/ \
    && rm -rf /tmp/openldap-${LDAP_VERSION}

USER 1001

