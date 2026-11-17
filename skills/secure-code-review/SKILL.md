---
name: secure-code-review
description: Professional skills and methodology for secure code review
version: 1.0.0
---

# Secure Code Review

## Overview

Secure code review identifies vulnerabilities in source code. This skill provides methods, tools, and best practices.

## Review Scope

### 1. Input Validation

**Items to check:**
- User input validation
- Parameter validation
- Data filtering
- Boundary checks

### 2. Output Encoding

**Items to check:**
- XSS prevention
- Output encoding
- Content Security Policy
- Response header configuration

### 3. Authentication and Authorization

**Items to check:**
- Authentication mechanisms
- Session management
- Access control
- Password handling

### 4. Cryptography and Keys

**Items to check:**
- Data encryption
- Key management
- Hash algorithms
- Random number generation

## Review Methods

### 1. Static Analysis

**Using SAST tools:**
```bash
# SonarQube
sonar-scanner

# Checkmarx
# Use the web interface

# Fortify
sourceanalyzer -b project build.sh
sourceanalyzer -b project -scan

# Semgrep
semgrep --config=auto .
```

### 2. Manual Review

**Review checklist:**
- [ ] Input validation
- [ ] Output encoding
- [ ] SQL injection
- [ ] XSS vulnerabilities
- [ ] Authentication and authorization
- [ ] Cryptography usage
- [ ] Error handling
- [ ] Logging

### 3. Code Pattern Recognition

**Dangerous functions:**
```python
# Dangerous Python functions
eval()
exec()
pickle.loads()
os.system()
subprocess.call()
```

```java
// Dangerous Java functions
Runtime.exec()
ProcessBuilder()
Class.forName()
```

```php
// Dangerous PHP functions
eval()
exec()
system()
passthru()
```

## Common Vulnerability Patterns

### SQL Injection

**Unsafe code:**
```java
String query = "SELECT * FROM users WHERE id = " + userId;
Statement stmt = connection.createStatement();
ResultSet rs = stmt.executeQuery(query);
```

**Safe code:**
```java
String query = "SELECT * FROM users WHERE id = ?";
PreparedStatement stmt = connection.prepareStatement(query);
stmt.setInt(1, userId);
ResultSet rs = stmt.executeQuery();
```

### XSS Vulnerabilities

**Unsafe code:**
```javascript
document.innerHTML = userInput;
element.innerHTML = "<div>" + userInput + "</div>";
```

**Safe code:**
```javascript
element.textContent = userInput;
element.setAttribute("data-value", userInput);
// Or use an encoding library
element.innerHTML = escapeHtml(userInput);
```

### Command Injection

**Unsafe code:**
```python
import os
os.system("ping " + user_input)
```

**Safe code:**
```python
import subprocess
subprocess.run(["ping", "-c", "1", validated_input])
```

### Path Traversal

**Unsafe code:**
```java
String filePath = "/uploads/" + fileName;
File file = new File(filePath);
```

**Safe code:**
```java
String basePath = "/uploads/";
String fileName = Paths.get(fileName).getFileName().toString();
String filePath = basePath + fileName;
File file = new File(filePath);
if (!file.getCanonicalPath().startsWith(basePath)) {
    throw new SecurityException("Invalid path");
}
```

### Hardcoded Keys

**Unsafe code:**
```java
String apiKey = "1234567890abcdef";
String password = "admin123";
```

**Safe code:**
```java
String apiKey = System.getenv("API_KEY");
String password = keyStore.getPassword("db_password");
```

## Tools

### SonarQube

```bash
# Start SonarQube
docker run -d -p 9000:9000 sonarqube

# Run the scan
sonar-scanner \
  -Dsonar.projectKey=myproject \
  -Dsonar.sources=. \
  -Dsonar.host.url=http://localhost:9000
```

### Semgrep

```bash
# Install
pip install semgrep

# Run the scan
semgrep --config=auto .

# Use rules
semgrep --config=p/security-audit .
```

### CodeQL

```bash
# Create the database
codeql database create database --language=java --source-root=.

# Run queries
codeql database analyze database security-and-quality.qls --format=sarif-latest
```

## Review Checklist

### Input Validation
- [ ] All user input is validated
- [ ] Allowlist validation is used
- [ ] Data types and ranges are validated
- [ ] Special characters are handled

### Output Encoding
- [ ] HTML output encoding
- [ ] URL encoding
- [ ] JavaScript encoding
- [ ] SQL parameterization

### Authentication and Authorization
- [ ] Strong password policies
- [ ] Secure session management
- [ ] Permission checks
- [ ] Multi-factor authentication

### Cryptography
- [ ] Strong encryption algorithms
- [ ] Secure key storage
- [ ] Transport encryption
- [ ] Storage encryption

### Error Handling
- [ ] No sensitive information disclosure
- [ ] Consistent error responses
- [ ] Error logging
- [ ] Exception handling

## Best Practices

### 1. Secure Coding Standards

- Follow the OWASP Top 10
- Use secure coding guidelines
- Establish code review procedures
- Provide security training

### 2. Automation Tools

- Integrate SAST tools
- Run CI/CD security checks
- Automate scanning
- Analyze results

### 3. Code Review Process

- Peer review
- Security expert review
- Regular reviews
- Record issues

## Considerations

- Combine tools with manual review
- Focus on business logic vulnerabilities
- Update tool rules regularly
- Build a secure coding culture