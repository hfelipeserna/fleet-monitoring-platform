# syntax=docker/dockerfile:1
FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG VITE_API_BASE_URL=""
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
RUN apk add --no-cache libcap wget \
 && addgroup -S app && adduser -S app -G app \
 && mkdir -p /var/cache/nginx /var/run/nginx /var/log/nginx /tmp \
 && chown -R app:app /var/cache/nginx /var/run/nginx /var/log/nginx /usr/share/nginx/html /etc/nginx/conf.d \
 && setcap cap_net_bind_service=+ep /usr/sbin/nginx \
 && touch /tmp/nginx.pid && chown app:app /tmp/nginx.pid \
 && sed -i 's|pid .*;|pid /tmp/nginx.pid;|' /etc/nginx/nginx.conf || true \
 && grep -q "pid /tmp/nginx.pid" /etc/nginx/nginx.conf || echo "pid /tmp/nginx.pid;" >> /etc/nginx/nginx.conf
USER app
EXPOSE 80
HEALTHCHECK --interval=5s --timeout=3s --retries=10 --start-period=5s CMD wget -qO- http://localhost:80/ || wget -qO- http://localhost:80/healthz || exit 1
CMD ["nginx", "-g", "daemon off;"]
