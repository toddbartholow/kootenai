#!/usr/bin/env python3
"""
Initialize Kootenai Database

Applies the database schema to PostgreSQL.

Usage:
    python3 init_database.py [--host HOST] [--drop-existing]
"""

import os
import sys
import argparse


def load_env():
    """Load environment variables from .env files."""
    try:
        from dotenv import load_dotenv
        for env_path in [".env.local", ".env", "../api/.env.local", "../api/.env", "api/.env.local", "api/.env"]:
            if os.path.exists(env_path):
                load_dotenv(env_path, override=True)
    except ImportError:
        pass


def main():
    parser = argparse.ArgumentParser(description="Initialize Kootenai Database")
    parser.add_argument("--host", default=None, help="Database host (default: from env)")
    parser.add_argument("--port", type=int, default=5432, help="Database port (default: 5432)")
    parser.add_argument("--user", default=None, help="Database user (default: from env)")
    parser.add_argument("--password", default=None, help="Database password (default: from env)")
    parser.add_argument("--database", default=None, help="Database name (default: from env)")
    parser.add_argument("--drop-existing", action="store_true", help="Drop existing tables first")
    parser.add_argument("--schema-file", default=None, help="Path to schema SQL file")
    args = parser.parse_args()

    load_env()

    # Get connection parameters
    host = args.host or os.getenv("DATABASE_HOST", "<INFRA_VM_IP>")
    port = args.port or int(os.getenv("DATABASE_PORT", "5432"))
    user = args.user or os.getenv("DATABASE_USER", "labadmin")
    password = args.password or os.getenv("DATABASE_PASSWORD")
    if not password:
        print("ERROR: DATABASE_PASSWORD environment variable is required")
        sys.exit(1)
    database = args.database or os.getenv("DATABASE_NAME", "virtuallab")

    # Find schema file
    schema_file = args.schema_file
    if not schema_file:
        possible_paths = [
            "docker/init-db/01_schema.sql",
            "../docker/init-db/01_schema.sql",
            "deploy/init-db/01_schema.sql",
            "../deploy/init-db/01_schema.sql",
        ]
        for path in possible_paths:
            if os.path.exists(path):
                schema_file = path
                break

    if not schema_file or not os.path.exists(schema_file):
        print("Error: Could not find schema file")
        print("Tried:", possible_paths if not args.schema_file else [args.schema_file])
        sys.exit(1)

    print("=" * 60)
    print("Kootenai Database Initialization")
    print("=" * 60)
    print(f"Host: {host}:{port}")
    print(f"Database: {database}")
    print(f"User: {user}")
    print(f"Schema file: {schema_file}")
    print("=" * 60)

    try:
        import psycopg2
        from psycopg2 import sql
    except ImportError:
        print("Error: psycopg2 not installed. Run: pip install psycopg2-binary")
        sys.exit(1)

    # Connect to database
    print("\nConnecting to database...")
    try:
        conn = psycopg2.connect(
            host=host,
            port=port,
            user=user,
            password=password,
            database=database
        )
        conn.autocommit = True
        cursor = conn.cursor()
        print("Connected successfully!")
    except Exception as e:
        print(f"Failed to connect: {e}")
        sys.exit(1)

    # Check if schema already exists
    cursor.execute("""
        SELECT EXISTS (
            SELECT FROM information_schema.tables
            WHERE table_schema = 'public'
            AND table_name = 'lab_templates'
        );
    """)
    schema_exists = cursor.fetchone()[0]

    if schema_exists:
        if args.drop_existing:
            print("\nDropping existing schema...")
            # Drop all tables in reverse dependency order
            drop_statements = [
                "DROP TABLE IF EXISTS audit_log CASCADE;",
                "DROP TABLE IF EXISTS grade_history CASCADE;",
                "DROP TABLE IF EXISTS assessment_results CASCADE;",
                "DROP TABLE IF EXISTS wazuh_agents CASCADE;",
                "DROP TABLE IF EXISTS events CASCADE;",
                "DROP TABLE IF EXISTS checkpoint_progress CASCADE;",
                "DROP TABLE IF EXISTS lab_sessions CASCADE;",
                "DROP TABLE IF EXISTS pods CASCADE;",
                "DROP TABLE IF EXISTS users CASCADE;",
                "DROP TABLE IF EXISTS lab_templates CASCADE;",
                "DROP TABLE IF EXISTS grade_sync_queue CASCADE;",
                "DROP TYPE IF EXISTS platform_type CASCADE;",
                "DROP TYPE IF EXISTS pod_status CASCADE;",
                "DROP TYPE IF EXISTS checkpoint_status CASCADE;",
                "DROP TYPE IF EXISTS trigger_type CASCADE;",
            ]
            for stmt in drop_statements:
                try:
                    cursor.execute(stmt)
                except Exception as e:
                    print(f"  Warning: {e}")
            print("Existing schema dropped.")
        else:
            print("\nSchema already exists. Use --drop-existing to recreate.")
            print("Checking existing tables...")
            cursor.execute("""
                SELECT table_name FROM information_schema.tables
                WHERE table_schema = 'public'
                ORDER BY table_name;
            """)
            tables = cursor.fetchall()
            print("Existing tables:")
            for table in tables:
                print(f"  - {table[0]}")
            conn.close()
            sys.exit(0)

    # Read and execute schema
    print("\nApplying schema...")
    with open(schema_file, 'r') as f:
        schema_sql = f.read()

    # Remove RAISE NOTICE which isn't supported outside functions
    schema_sql = schema_sql.replace("RAISE NOTICE", "-- RAISE NOTICE")

    try:
        cursor.execute(schema_sql)
        print("Schema applied successfully!")
    except Exception as e:
        print(f"Error applying schema: {e}")
        conn.close()
        sys.exit(1)

    # Verify tables were created
    print("\nVerifying tables...")
    cursor.execute("""
        SELECT table_name FROM information_schema.tables
        WHERE table_schema = 'public'
        ORDER BY table_name;
    """)
    tables = cursor.fetchall()
    print(f"Created {len(tables)} tables:")
    for table in tables:
        print(f"  - {table[0]}")

    # Check for seed data file
    seed_file = schema_file.replace("01_schema.sql", "02_seed_data.sql")
    if os.path.exists(seed_file):
        print(f"\nApplying seed data from {seed_file}...")
        with open(seed_file, 'r') as f:
            seed_sql = f.read()
        try:
            cursor.execute(seed_sql)
            print("Seed data applied successfully!")
        except Exception as e:
            print(f"Warning: Error applying seed data: {e}")

    conn.close()

    print("\n" + "=" * 60)
    print("Database initialization complete!")
    print("=" * 60)


if __name__ == "__main__":
    main()
