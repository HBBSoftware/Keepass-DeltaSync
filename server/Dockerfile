# SPDX-License-Identifier: AGPL-3.0-or-later
#
# DeltaSync sync-server — PHP 8.2 + Apache (mod_php).
#
# The app has NO third-party runtime dependencies (composer.json lists only
# dev deps) and ships its own PSR-4 autoloader in bootstrap.php, so there is
# no `composer install` / vendor/ step. We only need PHP plus a few extensions
# and the postgresql-client (used by the entrypoint to run schema migrations).
#
# State lives entirely in PostgreSQL (see compose.yml) — this image is
# stateless and can be rebuilt/replaced freely.

FROM php:8.2-apache

# --- PHP extensions ---------------------------------------------------------
# pdo_pgsql + sodium are the only non-bundled extensions the server needs
# (pdo, json are built in). libpq-dev/libsodium-dev pull in their runtime libs
# (libpq5/libsodium23) as dependencies, so we keep them for the linked .so to
# load at runtime. postgresql-client gives us psql + pg_isready for migrations.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        libpq-dev \
        libsodium-dev \
        postgresql-client; \
    docker-php-ext-install -j"$(nproc)" pdo_pgsql sodium; \
    rm -rf /var/lib/apt/lists/*

# --- Apache config ----------------------------------------------------------
# DocumentRoot → public/ ; .htaccess (mod_rewrite front-controller + dotfile
# hardening) needs AllowOverride All on that directory.
RUN set -eux; \
    sed -ri -e 's!/var/www/html!/var/www/app/public!g' /etc/apache2/sites-available/*.conf; \
    { \
        echo '<Directory /var/www/app/public>'; \
        echo '    AllowOverride All'; \
        echo '    Require all granted'; \
        echo '</Directory>'; \
    } > /etc/apache2/conf-available/deltasync.conf; \
    echo 'ServerName localhost' >> /etc/apache2/apache2.conf; \
    a2enmod rewrite headers; \
    a2enconf deltasync

# --- Application code --------------------------------------------------------
# Standard layout: app root holds bootstrap.php + src/ + schema/ + bin/, with
# public/ as the web root. index.php resolves the app root as "one dir up".
WORKDIR /var/www/app
COPY src/ ./src/
COPY public/ ./public/
COPY schema/ ./schema/
COPY bin/ ./bin/
COPY bootstrap.php ./bootstrap.php

# setup.php is the interactive web installer. The entrypoint performs schema
# migration and admin-token bootstrap non-interactively, so the installer is
# both unnecessary and extra attack surface — drop it from the image.
RUN rm -f ./public/setup.php

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh ./bin/admin

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["apache2-foreground"]
