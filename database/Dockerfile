# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# Use the official PostgreSQL image from Docker Hub
FROM postgres:latest

# Set environment variables for PostgreSQL
ENV POSTGRES_USER=main_user
ENV POSTGRES_PASSWORD=main_pass
ENV POSTGRES_DB=main

# Copy custom PostgreSQL configuration file
COPY postgresql.cnf /etc/postgresql/postgresql.conf

# Copy initialization scripts (schema.sql)
COPY schema.sql /docker-entrypoint-initdb.d/

# Ensure the custom configuration is used by overriding the default command
# CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]

# Start the PostgreSQL server with the custom configuration
CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
# CMD ["pg_ctl", "-D", "/var/lib/postgresql/data", "-l", "logfile", "start", "-o", "-c config_file=/etc/postgresql/postgresql.conf"]