# syntax=docker/dockerfile:1

# --- Stage 1: build -----------------------------------------------------------
FROM golang:1.26 AS build

WORKDIR /src

# Copia prima i soli file dei moduli: il layer di download delle dipendenze
# resta in cache finché go.mod/go.sum non cambiano.
COPY go.mod go.sum ./
RUN go mod download

# Copia il resto del sorgente e compila un binario statico, senza CGO, per Linux.
# -trimpath rimuove i path locali dal binario; -s -w riducono la dimensione.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server

# --- Stage 2: runtime ---------------------------------------------------------
# distroless static: nessuna shell, nessun package manager, utente non-root.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /server /server

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
