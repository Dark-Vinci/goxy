# Dockerfile
FROM postgres:16

# Optional: Set default timezone
ENV TZ=UTC

# Environment variables for default database, user, and password
# (can also be overridden in docker-compose.yml)
ENV POSTGRES_DB=appdb
ENV POSTGRES_USER=appuser
ENV POSTGRES_PASSWORD=apppass

# Copy initialization scripts if you want to pre-load data
# COPY init.sql /docker-entrypoint-initdb.d/
