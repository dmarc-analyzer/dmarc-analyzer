# DMARC Analyzer

DMARC Analyzer is a tool for processing and analyzing DMARC (Domain-based Message Authentication, Reporting, and Conformance) reports. It helps organizations monitor email authentication results and protect their domains from email spoofing and phishing attacks.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Environment Variables](#environment-variables)
- [Development Setup](#development-setup)
- [AWS Service Configuration](#aws-service-configuration)
- [API Documentation](#api-documentation)
- [Deployment](#deployment)

## Overview

DMARC Analyzer processes DMARC aggregate reports that are stored in an S3 bucket. It parses these reports, extracts relevant information, and stores the data in a PostgreSQL database for analysis and visualization.

## Prerequisites

- Go 1.18 or later
- PostgreSQL 14 or later
- AWS account with S3 access
- Docker and Docker Compose (for containerized deployment)

## Environment Variables

The application requires the following environment variables:

```
# Database Configuration
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/dmarcdb

# AWS Configuration
S3_BUCKET_NAME=your-dmarc-reports-bucket
AWS_ACCESS_KEY_ID=your-aws-access-key
AWS_SECRET_ACCESS_KEY=your-aws-secret-key
AWS_REGION=your-aws-region
```

## Development Setup

### 1. Clone the Repository

```sh
git clone https://github.com/dmarc-analyzer/dmarc-analyzer.git
cd dmarc-analyzer
```

### 2. Set Up the Database

```sh
# Create the PostgreSQL database
createdb dmarcdb

# Apply the existing schema to your database
psql -d dmarcdb -f backend/schema.sql
```

### 3. Regenerating Database Schema (Advanced)

This step is only necessary when you've modified the model classes and need to regenerate the schema.sql file.

```sh
# Generate a new database schema
dropdb --if-exists gen_sql && createdb gen_sql
go run ./backend/cmd/generate_sql.go
echo '-- Code generated by dmarc-analyzer generate_sql. DO NOT EDIT.' > backend/schema.sql
pg_dump -d gen_sql --schema-only --no-owner | sed '/^--/d' | sed '/^SET /d' | sed '/^SELECT /d' | sed 's/public\.//g' | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}' -e 's/\n\n*/\n/' >> backend/schema.sql
dropdb --if-exists gen_sql

# Apply the newly generated schema to your database
psql -d dmarcdb -f backend/schema.sql
```

### 4. Configure Environment Variables

Create a `.env` file in the project root with the required environment variables as listed above.

### 5. Run the Application

```sh
# Start the server
go run ./backend/cmd/server/server.go
```

The server will start on port 6767 by default.

## AWS Service Configuration

### S3 Bucket Setup

1. Create an S3 bucket to store DMARC reports:
   - Sign in to the AWS Management Console
   - Navigate to S3 service
   - Click "Create bucket"
   - Enter a unique bucket name
   - Configure bucket settings as needed
   - Click "Create bucket"

2. Configure IAM permissions:
   - Create an IAM user or role with the following permissions:
     ```json
     {
       "Version": "2012-10-17",
       "Statement": [
         {
           "Effect": "Allow",
           "Action": [
             "s3:GetObject",
             "s3:ListBucket"
           ],
           "Resource": [
             "arn:aws:s3:::your-dmarc-reports-bucket",
             "arn:aws:s3:::your-dmarc-reports-bucket/*"
           ]
         }
       ]
     }
     ```

3. Obtain AWS credentials (Access Key ID and Secret Access Key) for the IAM user.

### Receiving DMARC Reports

To receive DMARC reports in your S3 bucket, you need to:

1. Set up a DMARC record for your domain with the appropriate reporting address
2. Configure AWS SES to receive emails and store them in your S3 bucket

Example DMARC record:
```
_dmarc.example.com. IN TXT "v=DMARC1; p=none; rua=mailto:dmarc-reports@example.com;"
```

## Backfilling Reports

To process existing DMARC reports in your S3 bucket:

```sh
go run ./backend/cmd/backfill/backfill.go
```

This command will scan your S3 bucket for DMARC reports, parse them, and store the data in the PostgreSQL database.

## API Documentation

The DMARC Analyzer provides the following API endpoints:

### List Domains

```sh
curl http://127.0.0.1:6767/api/domains
```

Returns a list of all domains with DMARC reports.

### Domain Summary Report

```sh
curl http://127.0.0.1:6767/api/domains/example.com/report?start=2024-10-10T00:00:00Z&end=2024-10-20T00:00:00Z
```

Returns a summary of DMARC reports for the specified domain and date range.

### Domain Detail Report

```sh
curl http://127.0.0.1:6767/api/domains/example.com/report/detail?start=2024-10-10T00:00:00Z&end=2024-10-20T00:00:00Z
```

Returns detailed DMARC report information for the specified domain and date range.

### Domain DMARC Chart Data

```sh
curl http://127.0.0.1:6767/api/domains/example.com/chart/dmarc?start=2024-10-10T00:00:00Z&end=2024-10-20T00:00:00Z
```

Returns data for generating DMARC compliance charts for the specified domain and date range.

## Deployment

### Using Docker Compose

1. Make sure Docker and Docker Compose are installed on your system.

2. Configure environment variables in the `docker-compose.yml` file or create a `.env` file in the project root.

3. Build and start the containers:

```sh
docker-compose up -d
```

This will start the DMARC Analyzer server and PostgreSQL database in containers.

### Manual Deployment

1. Build the application:

```sh
go build -o dmarc-server ./backend/cmd/server/server.go
```

2. Set up the PostgreSQL database and apply the schema as described in the Development Setup section.

3. Configure environment variables.

4. Run the server:

```sh
./dmarc-server
```

## License

See the [LICENSE](LICENSE) file for details.