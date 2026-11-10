# Ambiente do trabalho: Go na mesma versão da correção, com o servidor, o
# corretor e os bots de referência no PATH. Os executáveis já vêm compilados
# em bin/amd64 e bin/arm64; o docker escolhe a pasta da arquitetura da máquina.
#
#   docker build -t despacho .
#   docker run --rm -it -v "$PWD":/trabalho despacho

ARG GO_VERSAO=1.26.8
FROM docker.io/library/golang:${GO_VERSAO}-alpine

ARG TARGETARCH
COPY bin/${TARGETARCH}/ /usr/local/bin/

# Mesmas variáveis que o corretor usa ao compilar a entrega.
ENV GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-buildvcs=false

WORKDIR /trabalho
CMD ["sh"]
