# Imagem do Pursuit em duas etapas: a primeira compila, a segunda leva só o binário.
# A final não tem shell, gerenciador de pacotes nem nada além do executável (~15 MB).

FROM golang:1.27-alpine AS compilacao
WORKDIR /src
# Primeiro só as dependências: enquanto go.mod e go.sum não mudarem, o Docker reaproveita a camada
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
# Estático (sem cgo), sem caminhos da máquina e sem tabela de símbolos: binário menor
ARG COMMIT=desconhecido
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /pursuit ./cmd/pursuit

# distroless "static": só os certificados (o Telegram é HTTPS) e um usuário sem privilégios
FROM gcr.io/distroless/static-debian12:nonroot
ARG COMMIT=desconhecido
LABEL org.opencontainers.image.revision=$COMMIT
COPY --from=compilacao /pursuit /pursuit
USER nonroot
# O Go devolve memória ao chegar perto disso (o contêiner tem 64 MB; a VM tem 1 GB para tudo)
ENV PURSUIT_ENDERECO=:8095 GOMEMLIMIT=48MiB
EXPOSE 8095
# Sem curl nem wget na imagem: o próprio binário confere a saúde
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/pursuit", "saude"]
ENTRYPOINT ["/pursuit"]
