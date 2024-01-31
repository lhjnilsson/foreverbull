#!/bin/bash

set -e
set -u

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
	CREATE USER foreverbull WITH PASSWORD 'foreverbull';
	ALTER ROLE foreverbull Superuser;

	CREATE DATABASE foreverbull;
	GRANT ALL PRIVILEGES ON DATABASE foreverbull TO foreverbull;
	ALTER DATABASE foreverbull OWNER TO foreverbull;

	CREATE DATABASE foreverbull_testing;
	    GRANT ALL PRIVILEGES ON DATABASE foreverbull_testing TO foreverbull;
	    ALTER DATABASE foreverbull_testing OWNER TO foreverbull;
EOSQL
