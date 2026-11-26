FROM node:26.13.1-alpine@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5 AS build

WORKDIR /app

RUN apk update && \
    apk add pnpm --no-cache

ARG VITE_BACKEND_URL
ENV VITE_BACKEND_URL=${VITE_BACKEND_URL}

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build


FROM nginx/nginx-unprivileged:1.31.10-alpine-slim@sha256:f4afc48254dc77ea692ec6979d220e5b1176331c636a4d51c502ffdded6789bd

USER root

RUN mkdir -p /tmp/nginx && \
    chown -R nginx:nginx /tmp/nginx

USER nginx

COPY --from=build /app/dist /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 3000
