FROM archlinux:latest

RUN pacman -Syu --noconfirm

RUN pacman -Sy --noconfirm \
    git \
    gcc \
    go \
    npm \
    jdk-openjdk \
    zip \
    which

RUN go install github.com/nathants/cli-aws@latest && \
    mv -fv ~/go/bin/* /usr/local/bin
