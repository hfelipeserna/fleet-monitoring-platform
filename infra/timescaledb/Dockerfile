FROM timescale/timescaledb:latest-pg18
RUN apk add --no-cache postgis && \
    mkdir -p /usr/local/share/postgresql/extension /usr/local/lib/postgresql && \
    cp /usr/share/postgresql18/extension/postgis* /usr/local/share/postgresql/extension/ 2>/dev/null || true && \
    cp /usr/share/postgresql18/extension/address_standardizer* /usr/local/share/postgresql/extension/ 2>/dev/null || true && \
    mkdir -p /usr/local/share/postgresql/contrib/postgis-3.6 /usr/local/share/postgresql/contrib/postgis-3.5 && \
    cp -r /usr/share/postgresql18/contrib/postgis-3.6/* /usr/local/share/postgresql/contrib/postgis-3.6/ 2>/dev/null || true && \
    cp -r /usr/share/postgresql18/contrib/postgis-3.6/* /usr/local/share/postgresql/contrib/postgis-3.5/ 2>/dev/null || true && \
    cp /usr/lib/postgresql18/postgis*.so /usr/local/lib/postgresql/ 2>/dev/null || true && \
    cp /usr/lib/postgresql18/postgis*.so /usr/lib/postgresql/ 2>/dev/null || true && \
    cp -r /usr/lib/postgresql18/bitcode/postgis* /usr/local/lib/postgresql/bitcode/ 2>/dev/null || true
