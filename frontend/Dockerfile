FROM node:24.20.0-alpine@sha256:e67514e5d0f6c46656005e1b693b2ec9d52e80b641307de684d4a015ba7a4eaf AS build

WORKDIR /app

RUN apk update && \
    apk add pnpm --no-cache

ARG VITE_BACKEND_URL
ENV VITE_BACKEND_URL=${VITE_BACKEND_URL}

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build


FROM nginxinc/nginx-unprivileged:1.31.4-alpine-slim@sha256:d668aa123a6ec3216ba5ae6b398ae8001d5e81d3142d3659e20354fd0c3c3125

USER root

RUN mkdir -p /tmp/nginx && \
    chown -R nginx:nginx /tmp/nginx

USER nginx

COPY --from=build /app/dist /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 3000
