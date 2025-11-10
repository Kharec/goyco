#!/bin/bash
# helper script to setup a postgres database on deb based systems

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

read -s "Do you want to install PostgreSQL 17? [y/N] " INSTALL_PG
if [ "$INSTALL_PG" != "y" ]; then
    echo "PostgreSQL 17 will not be installed"
    exit 0
fi

read -s -p "Enter password for PostgreSQL user 'goyco': " GOYCO_PWD
echo

apt-get update
apt-get install -y postgresql-17 

systemctl enable --now postgresql

su - postgres -c "psql" <<EOF
DO
\$body\$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_user WHERE usename = 'goyco') THEN
      CREATE USER goyco WITH PASSWORD '${GOYCO_PWD}';
   END IF;
END
\$body\$;

DO
\$body\$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_database WHERE datname = 'goyco') THEN
      CREATE DATABASE goyco OWNER goyco;
   END IF;
END
\$body\$;

-- Give all privileges on database to goyco (redundant if owner, but explicit)
GRANT ALL PRIVILEGES ON DATABASE goyco TO goyco;
EOF

echo "PostgreSQL 17 installed, database 'goyco' and user 'goyco' set up."


