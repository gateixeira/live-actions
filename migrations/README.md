# Database Migrations

This directory contains database migrations for the R-Pulse application.

## Prerequisites

The application requires PostgreSQL.

## Migration Files

Migrations follow the format: `{version}_{description}.{up|down}.sql`

Example:

- 000001_create_initial_schema.up.sql
- 000001_create_initial_schema.down.sql

## Creating New Migrations

Use the following format for new migration files:

1. Increment the version number
2. Add descriptive name
3. Create both up and down migrations
