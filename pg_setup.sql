CREATE USER inventory_api with password 'postgres';
GRANT ALL PRIVILEGES ON DATABASE inventory TO inventory_api;
ALTER USER inventory_api WITH SUPERUSER;

-- TODO SEE local_db_setup.sh for iterate