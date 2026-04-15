FROM node:22-alpine AS base
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev && npm cache clean --force

FROM node:22-alpine
WORKDIR /app
RUN apk add --no-cache tini
COPY --from=base /app/node_modules ./node_modules
COPY package.json tsconfig.json ./
COPY src/ ./src/

ENV MCP_TRANSPORT=http
ENV MCP_PORT=8080
EXPOSE 8080

ENTRYPOINT ["tini", "--"]
CMD ["npx", "tsx", "src/index.ts"]
