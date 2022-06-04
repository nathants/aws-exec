FROM archlinux:latest

RUN pacman -Syu --noconfirm

RUN pacman -Sy --noconfirm \
    entr \
    gcc \
    git \
    go \
    jdk-openjdk \
    npm \
    which \
    zip

RUN go install github.com/nathants/libaws@latest && \
    mv -fv ~/go/bin/* /usr/local/bin
