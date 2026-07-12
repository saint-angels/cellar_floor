# Build the client
FROM node:20-alpine AS client
WORKDIR /src/client
COPY client/package.json client/package-lock.json ./
RUN npm ci
COPY client/ ./
RUN npm run build

# Build the server
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o /cellarfloor ./cmd/cellarfloor

# Runtime: the workdir is a mounted volume, so the cwd-relative save_path
# from data/sim.toml lands world.json and players.json on the host
FROM alpine:3.20
COPY --from=server /cellarfloor /app/cellarfloor
COPY data/ /app/data/
COPY --from=client /src/client/dist /app/dist/
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/app/cellarfloor", "-data", "/app/data", "-static", "/app/dist"]
