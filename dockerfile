# --- ÉTAPE 1 : Compilation ---
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Dépendances
COPY go.mod ./
# COPY go.sum ./ (décommente si le fichier existe)
RUN go mod download

COPY . .

# Compilation du binaire Power-4
RUN CGO_ENABLED=0 GOOS=linux go build -o power4 .

# --- ÉTAPE 2 : Image de Production ---
FROM alpine:latest

WORKDIR /root/

# Récupération du binaire
COPY --from=builder /app/power4 .

# Récupération des assets (On est maniaque : on prend tout ce qui sert au rendu)
COPY --from=builder /app/templates ./templates
# On copie aussi les fichiers racines si ton code les cherche là
COPY --from=builder /app/style.css .
COPY --from=builder /app/favicon.svg .

# Configuration du port unique

EXPOSE 80

CMD ["./power4"]